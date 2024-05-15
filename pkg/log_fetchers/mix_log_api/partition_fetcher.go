package mix_log_api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	ct "nuance.xaas-logging.event-log-collector/pkg/log_fetchers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	t "nuance.xaas-logging.event-log-collector/pkg/types"
)

// Batch represents a set of records returned as part of a log.api fetch request
type Batch struct {
	created time.Time
	size    int
	records []interface{}
}

// PartitionFetcher wraps the execution of fetching and processing records for a given partition
// This design helps simplify signaling of shutdown requests and ensure each partition gracefully
// completes full processing of a record batch and monitoring of that processing
type PartitionFetcher struct {
	logFetcher *LogFetcher  // An instance of the main LogFetcher
	consumer   *ct.Consumer // The consumer for the given partition

	batch chan Batch

	shutdown      chan bool
	handlerSignal chan bool
	monitorSignal chan bool
}

// NewPartitionFetcher() creates a new PartitionFetcher and launches background monitoring and batch processing services
func NewPartitionFetcher(LogFetcher *LogFetcher, consumer *ct.Consumer, shutdown chan bool) *PartitionFetcher {
	pf := &PartitionFetcher{
		logFetcher:    LogFetcher,
		consumer:      consumer,
		batch:         make(chan Batch),
		shutdown:      shutdown,
		handlerSignal: make(chan bool),
		monitorSignal: make(chan bool),
	}

	go pf.monitorPartition()
	go pf.handler()

	return pf
}

//// Private Methods

func (pf *PartitionFetcher) commitOffset(offset int) {
	// only commit if auto commit is not enabled
	if !pf.logFetcher.autoCommitEnabled && offset >= 0 {
		log.Debugf("committing offset to %v for partition %v", offset, pf.consumer.Partition)
		if err := pf.logFetcher.commitOffset(*pf.consumer, offset); err != nil {
			log.Errorf("Partition [%v]: failed to commit offset [%v]: %v", pf.consumer.Partition, offset, err)
		}
	}
}

// randomSleep() causes partition fetcher to sleep for a random number of seconds, between 0 and 60
func (pf *PartitionFetcher) randomSleep() {
	// sleep a random number of seconds and then try again
	delay, _ := rand.Int(rand.Reader, big.NewInt(60))
	log.Infof("fetcher rate limited - sleeping %v seconds", delay)
	time.Sleep(time.Duration(delay.Add(delay, big.NewInt(1)).Int64()) * time.Second)
}

// monitorPartition() periodically checks the parition's offset details so backlog can be monitored
func (pf *PartitionFetcher) monitorPartition() {
	for {
		select {
		case <-pf.monitorSignal:
			log.Debugf("Stopped monitoring partition %v for consumer [%s]", pf.consumer.Partition, pf.consumer.Name)
			return
		case <-time.After(1 * time.Minute):
			offsetDetails, err := pf.logFetcher.getOffsetDetails(*pf.consumer)
			if err != nil {
				log.Errorf("%v", err)
			} else {
				pf.logFetcher.logOffsetDetails(*offsetDetails)
			}
		}
	}
}

// handler() processes each batch of fetched records, writing records to the data pipeline to be picked up by downstream components (processor, writer)
func (pf *PartitionFetcher) handler() {
	var processingComplete sync.WaitGroup

	for {
		select {
		case <-pf.handlerSignal:
			log.Debugf("Waiting for partition record handler %v to complete processing batch...", pf.consumer.Partition)
			processingComplete.Wait()
			pf.monitorSignal <- true
			log.Debugf("Partition record handler has finish processing batch for partition %v", pf.consumer.Partition)
			return
		case batch := <-pf.batch:
			// set a signal in case handler recieves a shutdown signal
			processingComplete.Add(1)
			processingStart := time.Now()

			offset := -1
			for _, r := range batch.records {
				bytes, err := json.Marshal(r)
				if err != nil {
					log.Errorf("%v", err)
					// monitor the error...
					monitoring.IncCounter(monitoring.ElcErrors,
						monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
						pf.logFetcher.topic,
						fmt.Sprint(monitoring.PROM_ERR_MARSHAL_ERROR),
						err.Error())

					monitoring.IncCounter(monitoring.ElcProcessingTotal,
						monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
						pf.logFetcher.topic,
						monitoring.PROM_STATUS_FAILED,
						fmt.Sprint(monitoring.PROM_ERR_PIPELINE_WRITE_ERROR),
						err.Error())

				} else {
					// for each record in the batch, write it out to the data pipeline...
					var eventLog t.EventLogSchema
					json.Unmarshal(bytes, &eventLog)
					dt, _ := time.Parse(time.RFC3339, eventLog.Value.Timestamp)
					offset = int(eventLog.Offset)

					// monitor lag in
					monitoring.SetGauge(monitoring.ElcLagIn, float64(time.Since(dt).Milliseconds()),
						monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
						pf.logFetcher.topic)

					if err := pf.logFetcher.writer.Write(bytes); err != nil {
						// monitor errors
						monitoring.IncCounter(monitoring.ElcErrors,
							monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
							pf.logFetcher.topic,
							fmt.Sprint(monitoring.PROM_ERR_PIPELINE_WRITE_ERROR),
							err.Error())

						monitoring.SetGauge(monitoring.ElcProcessingDuration, float64(time.Since(processingStart).Milliseconds()),
							monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
							pf.logFetcher.topic,
							monitoring.PROM_STATUS_FAILED,
							fmt.Sprint(monitoring.PROM_ERR_PIPELINE_WRITE_ERROR),
							err.Error())

						monitoring.IncCounter(monitoring.ElcProcessingTotal,
							monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
							pf.logFetcher.topic,
							monitoring.PROM_STATUS_FAILED,
							fmt.Sprint(monitoring.PROM_ERR_PIPELINE_WRITE_ERROR),
							err.Error())

						log.Fatalf("%v", err)
					}

					// monitor lag, duration, counters...
					monitoring.SetGauge(monitoring.ElcLagOut, float64(time.Since(dt).Milliseconds()),
						monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
						pf.logFetcher.topic)

					monitoring.SetGauge(monitoring.ElcProcessingDuration, float64(time.Since(processingStart).Milliseconds()),
						monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
						pf.logFetcher.topic,
						monitoring.PROM_STATUS_SUCCESS,
						fmt.Sprint(http.StatusOK),
						monitoring.PROM_MSG_PROCESSED)

					monitoring.IncCounter(monitoring.ElcNumRecordsOutCounter,
						monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
						pf.logFetcher.topic)

					monitoring.IncCounter(monitoring.ElcProcessingTotal,
						monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
						pf.logFetcher.topic,
						monitoring.PROM_STATUS_SUCCESS,
						fmt.Sprint(http.StatusOK),
						monitoring.PROM_MSG_PROCESSED)
				}
			}

			// manually commit cursor if auto commit is not enabled
			pf.commitOffset(offset)

			// signal that batch is done being processed
			processingComplete.Done()
		}
	}
}

func (pf *PartitionFetcher) retryOrQuit(err error, errCount int) {
	maxRetries := 3

	// retry or quit?
	if errCount <= maxRetries {
		pf.randomSleep()
	} else {
		log.Fatalf("%v", err)
	}
}

// processApiErrorResponse() processes API responses containing a non-null err
func (pf *PartitionFetcher) processApiErrorResponse(err error, errCount int) {

	// monitor errors
	monitoring.IncCounter(monitoring.ElcErrors,
		monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
		pf.logFetcher.topic,
		fmt.Sprint(monitoring.PROM_ERR_FETCH_ERROR),
		err.Error())

	// retry or quit?
	pf.retryOrQuit(err, errCount)
}

// processApi200Response() processes successful API responses
func (pf *PartitionFetcher) processApi200Response(body []byte, totalRecords *int) {
	// unmarshall the response and check to see how many records were fetched
	var records []interface{}
	_ = json.Unmarshal(body, &records)
	numRecords := len(records)
	*totalRecords += numRecords
	log.Infof("Partition [%v]: fetched %v records for processing (total fetched = %v)", pf.consumer.Partition, numRecords, *totalRecords)

	// If records were fetched, create a batch and pass off to the handler
	if numRecords > 0 {
		// monitor num of records fetched
		monitoring.AddCounter(monitoring.ElcNumRecordsInCounter, float64(numRecords),
			monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
			pf.logFetcher.topic)

		batch := Batch{created: time.Now(), size: numRecords, records: records}
		pf.batch <- batch
	} else {
		// no records were fetched, so let's sleep before fetching again
		log.Infof("Partition [%v]: sleeping %v seconds", pf.consumer.Partition, pf.logFetcher.cfg.RecordCheckFrequency)
		time.Sleep(time.Duration(pf.logFetcher.cfg.RecordCheckFrequency) * time.Second)
	}
}

// processApi404Response() processes API 404 errors by attempting to reset the partition consumer
func (pf *PartitionFetcher) processApi404Response(body []byte, err error) {
	log.Errorf("%v", string(body))
	var e error
	pf.consumer, e = pf.logFetcher.ResetPartitionConsumer(pf.consumer.Partition)
	if e != nil {
		log.Fatalf("%v", err)
	}
	// sleep a random number of seconds and then try again
	pf.randomSleep()
}

// processApi429Response() processes API 429 rate limiting errors by sleeping a random number of seconds
func (pf *PartitionFetcher) processApi429Response(code int) {
	// monitor the error
	monitoring.IncCounter(monitoring.ElcErrors,
		monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
		pf.logFetcher.topic,
		fmt.Sprint(code),
		"event log fetcher rate limited by log.api")

	// sleep a random number of seconds and then try again
	pf.randomSleep()
}

// processApi503Response() processes API 503 errors by sleeping and then retrying for qualified errors
func (pf *PartitionFetcher) processApi503Response(code int, body []byte, err error, errCount int) {
	switch {
	case body == nil:
		pf.processUnhandledApiResponse(code, body, err)
	case strings.Contains(string(body), "upstream connect error or disconnect/reset before headers. reset reason: connection termination"):
		// monitor the error
		monitoring.IncCounter(monitoring.ElcErrors,
			monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
			pf.logFetcher.topic,
			fmt.Sprint(code),
			string(body))

		pf.retryOrQuit(err, errCount)
	default:
		pf.processUnhandledApiResponse(code, body, err)
	}
}

// processUnhandledApiResponse() processes unknown errors by monitoring them and then killing the fetcher process
func (pf *PartitionFetcher) processUnhandledApiResponse(code int, body []byte, err error) {
	// monitor error
	monitoring.IncCounter(monitoring.ElcErrors,
		monitoring.PROM_LABEL_ELC_COMPONENT_FETCHER,
		pf.logFetcher.topic,
		fmt.Sprint(code),
		string(body))
	log.Fatalf("Error [status code: %v]: %v, %v", code, err, string(body))
}

//// Public Methods

// FetchRecords() calls log.api to fetch event log data
func (pf *PartitionFetcher) FetchRecords(wg *sync.WaitGroup) error {
	defer wg.Done()

	totalRecords := 0
	errCount := 0

	for {
		select {
		case <-pf.shutdown:
			log.Debugf("consumer [%s] has stopped fetching records on partition %d", pf.consumer.Name, pf.consumer.Partition)
			pf.handlerSignal <- true
			return nil
		default:
			// Always authenticate before accessing log.api. A cached token will be used if still valid.
			token, err := pf.logFetcher.Authenticator.Authenticate()
			if err != nil {
				log.Fatalf("%s", err)
				//return err
			}

			// Call the api
			url := pf.logFetcher.parseURL(RecordsPath)
			code, body, err := pf.logFetcher.Http.Get(url, pf.logFetcher.header(*token, pf.consumer.Name))

			// Process the api response
			switch {
			case err != nil:
				errCount += 1
				pf.processApiErrorResponse(err, errCount)
			case code == 200: // successful fetch
				errCount = 0
				pf.processApi200Response(body, &totalRecords)
			case code == 404: // {"status_code": "404", "error": "<nil>", "body": "{"error_code":40403,"message":"Consumer instance not found."}"}
				pf.processApi404Response(body, err)
			case code == 429: // rate limited by log.api
				pf.processApi429Response(code)
			case code == 503:
				errCount += 1
				pf.processApi503Response(code, body, err, errCount)
			default: // http error
				pf.processUnhandledApiResponse(code, body, err)
			}
		}
	}
}
