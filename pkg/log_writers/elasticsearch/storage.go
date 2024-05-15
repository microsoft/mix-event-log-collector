package elasticsearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/ratelimiter"

	"nuance.xaas-logging.event-log-collector/pkg/workerpool"

	es "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

const (
	REMOTE_SERVICE_NAME  = "ElasticSearch"
	TICKER_TIMEOUT_MS    = 30000
	RECORD_CNT_THRESHOLD = 1000
)

// ElasticSearchBulkRequestRecords provides a thread-safe implementation of a buffer of elasticsearch docs
type ElasticSearchBulkRequestRecords struct {
	data       bytes.Buffer
	numRecords int
	mu         sync.Mutex
}

// Length() returns the number of items in the docs buffer
func (r *ElasticSearchBulkRequestRecords) Length() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	length := r.numRecords
	return length
}

// SizeInBytes() returns the size in bytes of the docs buffer
func (r *ElasticSearchBulkRequestRecords) SizeInBytes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.data.Len()
	return size
}

// Append() adds an event log record to the docs buffer
func (r *ElasticSearchBulkRequestRecords) Append(docID string, value []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := []byte(fmt.Sprintf(`{ "index" : { "_id" : "%v" } }%s`, docID, "\n"))
	data := append(value, "\n"...)

	r.data.Grow(len(meta) + len(data))
	r.data.Write(meta)
	r.data.Write(data)

	r.numRecords++
}

// Records() returns a copy of the docs buffer
func (r *ElasticSearchBulkRequestRecords) Records() bytes.Buffer {
	r.mu.Lock()
	defer r.mu.Unlock()

	records := make([]byte, r.data.Len())
	r.data.Read(records)
	return *bytes.NewBuffer(records)
}

// Clear() removes all entries from the docs buffer
func (r *ElasticSearchBulkRequestRecords) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.Reset()
	r.numRecords = 0
}

// InfoResponse represents the elasticsearch restful API Info response object
type InfoResponse struct {
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
	ClusterUUID string `json:"cluster_uuid"`
	Version     struct {
		Number                    string `json:"number"`
		MinimumWireCompatibility  string `json:"minimum_wire_compatibility_version"`
		MinimumIndexCompatibility string `json:"minimum_index_compatibility_version"`
	}
}

// bulkResponse represents the elasticsearch restful API bulk response object
type bulkResponse struct {
	Errors bool `json:"errors"`
	Items  []struct {
		Index struct {
			ID     string `json:"_id"`
			Result string `json:"result"`
			Status int    `json:"status"`
			Error  struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
				Cause  struct {
					Type   string `json:"type"`
					Reason string `json:"reason"`
				} `json:"caused_by"`
			} `json:"error"`
		} `json:"index"`
	} `json:"items"`
}

// ElasticSearchClient manages the writing of data to elasticsearch storage
type ElasticSearchClient struct {
	// elasticsearch client and client params
	client              *es.Client
	ElasticMajorVersion int
	DocumentType        string
	index               string
	Refresh             bool
	Pipeline            *string

	// thread-safe buffer of event log records (aka elasticsearch docs)
	records ElasticSearchBulkRequestRecords

	// signals to manage data commits and client shutdown
	ticker              *time.Ticker
	commitDoneSignal    sync.WaitGroup
	awaitShutdownSignal sync.WaitGroup
	done                chan bool

	// resources for scaling via worker pool
	workers *workerpool.WorkerPool
	job     chan interface{}
	record  chan interface{}

	// rate limiting and retry definitions
	rateLimiter *rate.Limiter
	maxRetries  int
	delay       int
}

//// Public methods

// NewStorage() instantiates a new instance of ElasticSearchClient
func NewStorage(config Config) wt.Storage {
	if config.DocType == "" {
		config.DocType = "mix3-record"
	}

	// Do some marshalling to convert the config to the elasticsearch client config struct
	var esConfig es.Config
	bytes, _ := json.Marshal(config)
	json.Unmarshal(bytes, &esConfig)

	// If TLSConfig is set, then we need to create a new http.Transport object
	if config.TLSConfig != nil {
		// Again, do some marshalling to convert the TLSConfig struct to the tls.Config struct
		var tlsConfig tls.Config
		bytes, _ = json.Marshal(config.TLSConfig)
		json.Unmarshal(bytes, &tlsConfig)
		esConfig.Transport = &http.Transport{
			TLSClientConfig: &tlsConfig,
		}
	}
	log.Debugf("elasticsearch config: %+v", log.MaskSensitiveData(esConfig))

	// create elasticsearch client
	client, err := es.NewClient(esConfig)
	if err != nil {
		log.Errorf("error in creating elasticsearch client: %s", err)
	}

	// ping elasticsearch to ensure it is up and running and we can connect to it
	resp, err := client.Ping()
	switch {
	case err != nil:
		log.Fatalf("error in pinging elasticsearch: %s", err)
	case resp.StatusCode >= 400:
		log.Fatalf("error in pinging elasticsearch: %s", resp.Status())
	default:
		log.Infof("elasticsearch ping response: %s", resp)
	}

	// get elasticsearch info to determine the major version
	var versionMajor int
	res, err := client.Info()
	switch {
	case err != nil:
		log.Fatalf("error in getting elasticsearch info: %s", err)
	case res.StatusCode >= 400:
		log.Fatalf("error in getting elasticsearch info: %s", res.Status())
	default:
		log.Debugf("elasticsearch info response: %v", res.String())

		var info InfoResponse
		if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
			log.Fatalf("error in decoding elasticsearch info response: %s", err)
		}
		log.Infof("elasticsearch version: %v", info.Version.Number)
		versionMajor, err = strconv.Atoi(strings.Split(info.Version.Number, ".")[0])
		if err != nil {
			log.Fatalf("error in parsing elasticsearch version: %s", err)
		}
	}

	// create instance of client
	ec := &ElasticSearchClient{
		client:              client,
		ElasticMajorVersion: versionMajor,
		DocumentType:        config.DocType,
		Refresh:             config.Refresh,
		Pipeline:            config.Pipeline,

		job:    make(chan interface{}),
		record: make(chan interface{}),
		done:   make(chan bool),

		ticker:      time.NewTicker(TICKER_TIMEOUT_MS * time.Millisecond),
		rateLimiter: ratelimiter.NewRateLimiter(config.RateLimit),
		maxRetries:  config.MaxRetries,
		delay:       config.Delay,
	}

	// set up worker pool
	numWorkers := DEFAULT_NUM_WORKERS
	if config.NumWorkers != 0 {
		numWorkers = config.NumWorkers
	}

	ec.workers = workerpool.NewWorkerPool("elasticsearch-workers", ec.job, numWorkers, ec.storageHandler)
	ec.workers.Start()

	// run commit handler in background
	go ec.commitHandler()

	return ec
}

//// Helpers...

// storageHandler() is called by a workerpool worker to process a job
func (e *ElasticSearchClient) storageHandler(request interface{}) {
	var err error
	job := request.(*Job)
	switch job.Action {
	case WRITE:
		// We pass the record to the commitHandler() which handles batching and
		// writing of records to elasticsearch
		start := time.Now()
		log.Debugf("writing to elasticsearch record chan...")
		e.record <- request
		log.Debugf("done writing to elasticsearch chan in %v ms...", time.Since(start).Milliseconds())
	case READ: // Read is not actually implemented...
		var data []byte
		data, err = e.Read(job.Index, job.DocID)
		log.Debugf("%v", string(data))
	case DELETE: // Delete is not actually implemented...
		err = e.Delete(job.Index, job.DocID)

	}
	if err != nil {
		log.Errorf("%v", err)
	}
}

// commitHandler() batches records and triggers a commit after max records have
// been added to the batch or after a timeout has been triggered
func (e *ElasticSearchClient) commitHandler() {
	lastCommit := time.Now()
	for {
		select {
		case record := <-e.record:
			if record != nil {
				// Add the record to the batch
				job := record.(*Job)

				e.records.Append(job.DocID, job.Body)

				// If we've hit max batch record size, commit the batch to elasticsearch
				if e.records.Length() >= RECORD_CNT_THRESHOLD {
					log.Infof("elasticsearch bulk write triggered by record count")
					log.Infof("committing batch [count: %v, queue: %v]", e.records.Length(), len(e.record))
					go e.commit(e.records.Records(), e.records.Length())
					e.records.Clear()
					lastCommit = time.Now()
				}
			}
		case <-e.ticker.C:
			log.Infof("elasticsearch bulk write triggered by elapsed time")
			if e.records.Length() == 0 {
				log.Infof("no records to commit")
			} else if time.Since(lastCommit).Milliseconds() >= TICKER_TIMEOUT_MS {
				go e.commit(e.records.Records(), e.records.Length())
				e.records.Clear()
				lastCommit = time.Now()
			}
		case <-e.done:
			if e.records.Length() > 0 {
				go e.commit(e.records.Records(), e.records.Length())
				e.records.Clear()
			}
			e.commitDoneSignal.Wait()
			e.awaitShutdownSignal.Done()
		}
	}
}

// waitMyTurn() checks in with the rate limiter and waits if rate limit has been exceeded
func (e *ElasticSearchClient) waitMyTurn() {
	ctx := context.Background()
	startWait := time.Now()

	// rate limiting ...
	e.rateLimiter.Wait(ctx)
	log.Debugf("rate limited for %f seconds", time.Since(startWait).Seconds())
}

// monitorCommitResults() wraps logging and monitoring of the commit results
func (e *ElasticSearchClient) monitorCommitResults(start time.Time, numErrors, numIndexed int) {
	// monitor processing duration
	monitoring.SetGauge(wt.WriteDurationGauge,
		time.Since(start).Seconds(),
		e.DocumentType)

	dur := time.Since(start).Milliseconds()
	if numErrors > 0 {
		log.Errorf(
			"Indexed [%v] documents with [%v] errors in %v ms (%d docs/sec)",
			numIndexed,
			numErrors,
			dur,
			int64((1000.0/float64(dur))*float64(numIndexed)),
		)
	} else if numIndexed > 0 {
		log.Infof(
			"Successfully indexed [%v] documents in %v ms (%d docs/sec)",
			numIndexed,
			dur,
			int64((1000.0/float64(dur))*float64(numIndexed)),
		)
	} else {
		log.Infof("numErrors: %d, numIndexed: %d", numErrors, numIndexed)
	}
}

// bulkRequestOptionsBuilder() returns a list of bulk request options
func (e *ElasticSearchClient) bulkRequestOptionsBuilder() []func(*esapi.BulkRequest) {
	var bulkRequests []func(*esapi.BulkRequest)

	// Always include the index
	bulkRequests = append(bulkRequests, e.client.Bulk.WithIndex(e.index))

	// For older versions of elasticsearch, include the document type
	if e.ElasticMajorVersion < 8 {
		bulkRequests = append(bulkRequests, e.client.Bulk.WithDocumentType(e.DocumentType))
	}

	// If a pipeline is specified, include it
	if e.Pipeline != nil {
		bulkRequests = append(bulkRequests, e.client.Bulk.WithPipeline(*e.Pipeline))
	}

	return bulkRequests
}

// callBulkApi() handles the write request to elasticsearch
func (e *ElasticSearchClient) callBulkApi(r *bytes.Reader) (*esapi.Response, error) {
	// Observe write duration in seoncds and set histogram and guage metric
	timer := prometheus.NewTimer(CommitDurationHistogram)

	// Calling the actual api
	res, err := e.client.Bulk(r, e.bulkRequestOptionsBuilder()...)

	// monitor how long it takes to commit records to elasticsearch
	CommitDurationHistogram.Observe(timer.ObserveDuration().Seconds())

	return res, err
}

// processNetworkFailure() handles retry logic and monitoring for network-related errors with elasticsearch
func (e *ElasticSearchClient) processNetworkFailure(retry int, numRecords int) error {
	if e.maxRetries >= 0 && retry > e.maxRetries {
		err := fmt.Errorf("Network Failure: retry [%d] exceeded max retries [%d]", retry, e.maxRetries)
		monitoring.AddCounter(wt.WritesTotalCounter, float64(numRecords), wt.LABEL_ELASTICSEARCH, wt.NOT_WRITTEN, err.Error())
		monitoring.AddCounter(CommitTotalCounter, float64(numRecords), wt.ERR_NETWORK, err.Error())
		log.Errorf("error in bulk writing : %s", err.Error())
		return err
	}
	time.Sleep(time.Duration(time.Duration(e.delay) * time.Second))
	return nil
}

// processBulkResponseCompleteFailure() handles logging and monitoring when the elasticsearch bulk api request fails completely
func (e *ElasticSearchClient) processBulkResponseCompleteFailure(res *esapi.Response, numRecords int, numErrors *int) error {
	// Total failure...
	*numErrors += numRecords

	// Decode the response body
	var raw map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		log.Errorf("Failure to to parse response body: %s", err)
		monitoring.AddCounter(wt.WritesTotalCounter, float64(numRecords), wt.LABEL_ELASTICSEARCH, wt.NOT_WRITTEN, fmt.Sprintf("%v %v", res.StatusCode, res.Status()))
		monitoring.AddCounter(CommitTotalCounter, float64(numRecords), fmt.Sprintf("%v", res.StatusCode), res.Status())
	} else {
		// Do our best to parse out the error message...
		var msg, reason string

		switch raw["error"].(type) {
		case string:
			msg = fmt.Sprintf("%d %s", res.StatusCode, raw["error"].(string))
			reason = raw["error"].(string)

		case map[string]interface{}:
			msg = fmt.Sprintf("%d %s", res.StatusCode, raw["error"].(map[string]interface{})["type"])
			reason = raw["error"].(map[string]interface{})["reason"].(string)

		default:
			msg = fmt.Sprintf("%d %v", res.StatusCode, raw["error"])
			reason = fmt.Sprintf("%v", raw["error"])
		}

		// Log and monitor the error
		monitoring.AddCounter(wt.WritesTotalCounter, float64(numRecords), wt.LABEL_ELASTICSEARCH, wt.NOT_WRITTEN, msg)
		monitoring.AddCounter(CommitTotalCounter, float64(numRecords), fmt.Sprintf("%v", res.StatusCode), reason)

		log.Errorf("%s: %s",
			msg,
			reason,
		)
	}
	return nil
}

// processBulkResponse() handles logging and monitoring when the elasticsearch bulk api request is successful (may contain partial failures)
func (e *ElasticSearchClient) processBulkResponse(res *esapi.Response, numIndexed, numErrors *int) error {
	var blk *bulkResponse
	if err := json.NewDecoder(res.Body).Decode(&blk); err != nil {
		log.Errorf("Failure to to parse response body: %s", err)
		return err
	}

	// If the whole request failed, print error and mark all documents as failed
	for _, d := range blk.Items {
		// ... so for any HTTP status above 201 ...
		//
		if d.Index.Status > 201 {
			// ... increment the error counter ...
			//
			*numErrors++
			monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_ELASTICSEARCH, wt.NOT_WRITTEN, strconv.Itoa(d.Index.Status))
			monitoring.IncCounter(CommitTotalCounter, strconv.Itoa(d.Index.Status), d.Index.Error.Reason)
			// ... and print the response status and error information ...
			log.Errorf("Error: [%d]: %s: %s: %s: %s",
				d.Index.Status,
				d.Index.Error.Type,
				d.Index.Error.Reason,
				d.Index.Error.Cause.Type,
				d.Index.Error.Cause.Reason,
			)
		} else {
			// ... otherwise increase the success counter.
			*numIndexed++
			monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_ELASTICSEARCH, wt.WRITTEN, "")
			monitoring.IncCounter(CommitTotalCounter, strconv.Itoa(d.Index.Status), res.Status())
		}
	}
	return nil
}

// commit() writes a batch of records to elasticsearch
func (e *ElasticSearchClient) commit(records bytes.Buffer, count int) (int, error) {
	e.commitDoneSignal.Add(1)
	defer e.commitDoneSignal.Done()

	start := time.Now()
	retry := 0
	numErrors := 0
	numIndexed := 0
	var err error

	r := bytes.NewReader(records.Bytes())

	// call the elasticsearch Bulk() api, and then process the results
out:
	for {
		// check in with rate limiter...
		e.waitMyTurn()

		// call the elasticsearch bulk write api
		var res *esapi.Response
		res, err = e.callBulkApi(r)

		switch {
		case err != nil || res == nil:
			retry++
			err = e.processNetworkFailure(retry, count)
			if err != nil {
				numErrors += count
				break out
			}
		case res.IsError():
			// Make sure we close the response body...
			defer res.Body.Close()
			err = e.processBulkResponseCompleteFailure(res, count, &numErrors)
			break out
		default:
			// Make sure we close the response body...
			defer res.Body.Close()
			err = e.processBulkResponse(res, &numIndexed, &numErrors)
			break out
		}
	}

	e.monitorCommitResults(start, numErrors, numIndexed)
	return numIndexed, err
}

// Read() reads a record from elasticsearch. Not implemented.
func (e *ElasticSearchClient) Read(key string, path string) ([]byte, error) {
	return nil, fmt.Errorf("not Implemented")
}

// Write() enqueues a record to be batch-written to elasticsearch
func (e *ElasticSearchClient) Write(key string, path string, data []byte) (int, error) {
	// wrap record as a job and pass to worker pool
	e.index = key
	e.job <- NewJob(WRITE, key, path, data)
	return len(data), nil
}

// Delete() deletes a record from elasticsearch. Not implemented.
func (e *ElasticSearchClient) Delete(key string, path string) error {
	return fmt.Errorf("not Implemented")
}

// Shutdown() signals the elasticsearch storage provider to shutdown
func (e *ElasticSearchClient) Shutdown() {
	e.awaitShutdownSignal.Add(1)
	e.workers.Stop()
	e.ticker.Stop()
	e.done <- true
	e.awaitShutdownSignal.Wait()
}
