package opensearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/ratelimiter"

	"nuance.xaas-logging.event-log-collector/pkg/workerpool"

	ops "github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

const (
	REMOTE_SERVICE_NAME  = "OpenSearch"
	TICKER_TIMEOUT_MS    = 30000
	RECORD_CNT_THRESHOLD = 1000
)

// OpenSearchBulkRequestRecords provides a thread-safe implementation of a buffer of opensearch docs
type OpenSearchBulkRequestRecords struct {
	data       bytes.Buffer
	numRecords int
	mu         sync.Mutex
}

// Length() returns the number of items in the docs buffer
func (r *OpenSearchBulkRequestRecords) Length() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	length := r.numRecords
	return length
}

// SizeInBytes() returns the size in bytes of the docs buffer
func (r *OpenSearchBulkRequestRecords) SizeInBytes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.data.Len()
	return size
}

// Append() adds an event log record to the docs buffer
func (r *OpenSearchBulkRequestRecords) Append(docID string, value []byte) {
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
func (r *OpenSearchBulkRequestRecords) Records() bytes.Buffer {
	r.mu.Lock()
	defer r.mu.Unlock()

	records := make([]byte, r.data.Len())
	r.data.Read(records)
	return *bytes.NewBuffer(records)
}

// Clear() removes all entries from the docs buffer
func (r *OpenSearchBulkRequestRecords) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.Reset()
	r.numRecords = 0
}

// bulkResponse represents the opensearch restful API response object
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

// OpenSearchClient manages the writing of data to opensearch storage
type OpenSearchClient struct {
	// opensearch client and client params
	client       *ops.Client
	DocumentType string
	index        string
	Refresh      bool
	Pipeline     *string

	// thread-safe buffer of event log records (aka opensearch docs)
	records OpenSearchBulkRequestRecords

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

// NewStorage() instantiates a new instance of OpenSearchClient
func NewStorage(config Config) wt.Storage {
	var client *ops.Client
	var err error

	// Do some marshalling/unmarshalling to convert the config struct to the opensearch config struct
	var opsConfig ops.Config
	bytes, _ := json.Marshal(config)
	json.Unmarshal(bytes, &opsConfig)

	// If TLSConfig is set, then we need to create a custom http.Transport object
	if config.TLSConfig != nil {
		// Again, do some marshalling/unmarshalling to convert the TLSConfig struct to the tls config struct
		var tlsConfig tls.Config
		bytes, _ = json.Marshal(config.TLSConfig)
		json.Unmarshal(bytes, &tlsConfig)
		opsConfig.Transport = &http.Transport{
			TLSClientConfig: &tlsConfig,
		}
	}
	log.Debugf("opensearch config: %+v", log.MaskSensitiveData(opsConfig))

	// Create the opensearch client
	client, err = ops.NewClient(opsConfig)
	if err != nil {
		log.Errorf("error in creating opensearch client: %s", err)
	}

	// ping opensearch to ensure it is up and running and we can connect to it
	resp, err := client.Ping()
	switch {
	case err != nil:
		log.Fatalf("error in pinging opensearch: %s", err)
	case resp.StatusCode >= 400:
		log.Fatalf("error in pinging opensearch: %s", resp.Status())
	default:
		log.Infof("opensearch ping response: %s", resp)
	}

	// create instance of client
	oc := &OpenSearchClient{
		client:       client,
		Refresh:      config.Refresh,
		DocumentType: config.IndexPrefix, // Re-use the index prefix as the document type for monitoring...
		Pipeline:     config.Pipeline,

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

	oc.workers = workerpool.NewWorkerPool("opensearch-workers", oc.job, numWorkers, oc.storageHandler)
	oc.workers.Start()

	// run commit handler in background
	go oc.commitHandler()

	return oc
}

//// Helpers...

// storageHandler() is called by a workerpool worker to process a job
func (o *OpenSearchClient) storageHandler(request interface{}) {
	var err error
	job := request.(*Job)
	switch job.Action {
	case WRITE:
		// We pass the record to the commitHandler() which handles batching and
		// writing of records to opensearch
		start := time.Now()
		log.Debugf("writing to opensearch record chan...")
		o.record <- request
		log.Debugf("done writing to opensearch chan in %v ms...", time.Since(start).Milliseconds())
	case READ: // Read is not actually implemented...
		var data []byte
		data, err = o.Read(job.Index, job.DocID)
		log.Debugf("%v", string(data))
	case DELETE: // Delete is not actually implemented...
		err = o.Delete(job.Index, job.DocID)

	}
	if err != nil {
		log.Errorf("%v", err)
	}
}

// commitHandler() batches records and triggers a commit after max records have
// been added to the batch or after a timeout has been triggered
func (o *OpenSearchClient) commitHandler() {
	lastCommit := time.Now()
	for {
		select {
		case record := <-o.record:
			if record != nil {
				// Add the record to the batch
				job := record.(*Job)

				o.records.Append(job.DocID, job.Body)

				// If we've hit max batch record size, commit the batch to opensearch
				if o.records.Length() >= RECORD_CNT_THRESHOLD {
					log.Infof("opensearch bulk write triggered by record count")
					log.Infof("committing batch [count: %v, queue: %v]", o.records.Length(), len(o.record))
					go o.commit(o.records.Records(), o.records.Length())
					o.records.Clear()
					lastCommit = time.Now()
				}
			}
		case <-o.ticker.C:
			log.Infof("opensearch bulk write triggered by elapsed time")
			if o.records.Length() == 0 {
				log.Infof("no records to commit")
			} else if time.Since(lastCommit).Milliseconds() >= TICKER_TIMEOUT_MS {
				go o.commit(o.records.Records(), o.records.Length())
				o.records.Clear()
				lastCommit = time.Now()
			}
		case <-o.done:
			if o.records.Length() > 0 {
				go o.commit(o.records.Records(), o.records.Length())
				o.records.Clear()
			}
			o.commitDoneSignal.Wait()
			o.awaitShutdownSignal.Done()
		}
	}
}

// waitMyTurn() checks in with the rate limiter and waits if rate limit has been exceeded
func (o *OpenSearchClient) waitMyTurn() {
	ctx := context.Background()
	startWait := time.Now()

	// rate limiting ...
	o.rateLimiter.Wait(ctx)
	log.Debugf("rate limited for %f seconds", time.Since(startWait).Seconds())
}

// monitorCommitResults() wraps logging and monitoring of the commit results
func (o *OpenSearchClient) monitorCommitResults(start time.Time, numErrors, numIndexed int) {
	// monitor processing duration
	monitoring.SetGauge(wt.WriteDurationGauge,
		time.Since(start).Seconds(),
		o.DocumentType)

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
func (o *OpenSearchClient) bulkRequestOptionsBuilder() []func(*opensearchapi.BulkRequest) {
	var bulkRequests []func(*opensearchapi.BulkRequest)

	// Always include the index
	bulkRequests = append(bulkRequests, o.client.Bulk.WithIndex(o.index))

	// If a pipeline is specified, include it
	if o.Pipeline != nil {
		bulkRequests = append(bulkRequests, o.client.Bulk.WithPipeline(*o.Pipeline))
	}

	return bulkRequests
}

// callBulkApi() handles the write request to opensearch
func (o *OpenSearchClient) callBulkApi(r *bytes.Reader) (*opensearchapi.Response, error) {
	// Observe write duration in seoncds and set histogram and guage metric
	timer := prometheus.NewTimer(CommitDurationHistogram)

	// Calling the actual api...
	res, err := o.client.Bulk(r, o.bulkRequestOptionsBuilder()...)

	// monitor how long it takes to commit records to opensearch
	CommitDurationHistogram.Observe(timer.ObserveDuration().Seconds())

	return res, err
}

// processNetworkFailure() handles retry logic and monitoring for network-related errors with opensearch
func (o *OpenSearchClient) processNetworkFailure(retry int, numRecords int) error {
	if o.maxRetries >= 0 && retry > o.maxRetries {
		err := fmt.Errorf("Network Failure: retry [%d] exceeded max retries [%d]", retry, o.maxRetries)
		monitoring.AddCounter(wt.WritesTotalCounter, float64(numRecords), wt.LABEL_ELASTICSEARCH, wt.NOT_WRITTEN, err.Error())
		monitoring.AddCounter(CommitTotalCounter, float64(numRecords), wt.ERR_NETWORK, err.Error())
		log.Errorf("error in bulk writing : %s", err.Error())
		return err
	}
	time.Sleep(time.Duration(time.Duration(o.delay) * time.Second))
	return nil
}

// processBulkResponseCompleteFailure() handles logging and monitoring when the opensearch bulk api request fails completely
func (o *OpenSearchClient) processBulkResponseCompleteFailure(res *opensearchapi.Response, numRecords int, numErrors *int) error {
	// Total failure...
	*numErrors += numRecords

	// Decode the response body
	var raw map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		log.Errorf("Failure to to parse response body: %s", err)
		monitoring.AddCounter(wt.WritesTotalCounter, float64(numRecords), wt.LABEL_ELASTICSEARCH, wt.NOT_WRITTEN, fmt.Sprintf("%v %v", res.StatusCode, res.Status()))
		monitoring.AddCounter(CommitTotalCounter, float64(numRecords), fmt.Sprintf("%v", res.StatusCode), res.Status())
	} else {
		// Note that opensearch error body has a different structure than elasticsearch (string vs. map[string]interface{})
		msg := fmt.Sprintf("%d %s", res.StatusCode, raw["error"].(string))
		monitoring.AddCounter(wt.WritesTotalCounter, float64(numRecords), wt.LABEL_ELASTICSEARCH, wt.NOT_WRITTEN, msg)
		monitoring.AddCounter(CommitTotalCounter, float64(numRecords), fmt.Sprintf("%v", res.StatusCode), raw["error"].(string))

		log.Errorf("%s", msg)
	}
	return nil
}

// processBulkResponse() handles logging and monitoring when the opensearch bulk api request is successful (may contain partial failures)
func (o *OpenSearchClient) processBulkResponse(res *opensearchapi.Response, numIndexed, numErrors *int) error {
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

// commit() writes a batch of records to opensearch
func (o *OpenSearchClient) commit(records bytes.Buffer, count int) (int, error) {
	o.commitDoneSignal.Add(1)
	defer o.commitDoneSignal.Done()

	start := time.Now()
	retry := 0
	numErrors := 0
	numIndexed := 0
	var err error

	r := bytes.NewReader(records.Bytes())

	// call the opensearch Bulk() api, and then process the results
out:
	for {
		// check in with rate limiter...
		o.waitMyTurn()

		// call the opensearch bulk write api
		var res *opensearchapi.Response
		res, err = o.callBulkApi(r)

		switch {
		case err != nil || res == nil:
			retry++
			err = o.processNetworkFailure(retry, count)
			if err != nil {
				numErrors += count
				break out
			}
		case res.IsError():
			// Make sure we close the response body...
			defer res.Body.Close()
			err = o.processBulkResponseCompleteFailure(res, count, &numErrors)
			break out
		default:
			// Make sure we close the response body...
			defer res.Body.Close()
			err = o.processBulkResponse(res, &numIndexed, &numErrors)
			break out
		}
	}

	o.monitorCommitResults(start, numErrors, numIndexed)
	return numIndexed, err
}

// Read() reads a record from opensearch. Not implemented.
func (e *OpenSearchClient) Read(key string, path string) ([]byte, error) {
	return nil, fmt.Errorf("not Implemented")
}

// Write() enqueues a record to be batch-written to opensearch
func (o *OpenSearchClient) Write(key string, path string, data []byte) (int, error) {
	// wrap record as a job and pass to worker pool
	o.index = key
	o.job <- NewJob(WRITE, key, path, data)
	return len(data), nil
}

// Delete() deletes a record from opensearch. Not implemented.
func (o *OpenSearchClient) Delete(key string, path string) error {
	return fmt.Errorf("not Implemented")
}

// Shutdown() signals the opensearch storage provider to shutdown
func (o *OpenSearchClient) Shutdown() {
	o.awaitShutdownSignal.Add(1)
	o.workers.Stop()
	o.ticker.Stop()
	o.done <- true
	o.awaitShutdownSignal.Wait()
}
