package mix_log_producer_api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"nuance.xaas-logging.event-log-collector/pkg/httpclient"
	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/oauth"
	"nuance.xaas-logging.event-log-collector/pkg/ratelimiter"
	"nuance.xaas-logging.event-log-collector/pkg/types"
	"nuance.xaas-logging.event-log-collector/pkg/workerpool"
)

const (
	REMOTE_SERVICE_NAME         = "mix-log-producer-api"
	TICKER_TIMEOUT_MS           = 30000
	PAYLOAD_MAX_BYTES_THRESHOLD = int(9 * 1e+6) // 10MB per api spec. Setting to 9MB simplifies logic
)

// MixLogProducerApiRequestRecord represents a single entry in the list of records uploaded to Mix Log API
type MixLogProducerApiRequestRecord struct {
	Key   types.EventLogKey   `json:"key,omitempty"`
	Value types.EventLogValue `json:"value,omitempty"`
}

// MixLogProducerApiRequestRecords provides a thread-safe implementation of an array of MixLogProducerApiRequestRecord
type MixLogProducerApiRequestRecords struct {
	records  []MixLogProducerApiRequestRecord
	numBytes int
	mu       sync.Mutex
}

// Length() returns the number of items in the array of records
func (r *MixLogProducerApiRequestRecords) Length() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	length := len(r.records)
	return length
}

// SizeInBytes() returns the size in bytes of the array of records
func (r *MixLogProducerApiRequestRecords) SizeInBytes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.numBytes
	return size
}

// Append() adds a MixLogProducerApiRequestRecord to the array of records
func (r *MixLogProducerApiRequestRecords) Append(item MixLogProducerApiRequestRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bytes, _ := json.Marshal(item)
	r.numBytes += len(bytes)
	r.records = append(r.records, item)
}

// Records() returns a copy of the array of records
func (r *MixLogProducerApiRequestRecords) Records() []MixLogProducerApiRequestRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]MixLogProducerApiRequestRecord, len(r.records))
	copy(records, r.records)
	return records
}

// Clear() removes all entries from the array of records
func (r *MixLogProducerApiRequestRecords) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = nil
	r.numBytes = 0
}

// MixLogProducerApiRequest represents a request object to be passed to Mix Log API
type MixLogProducerApiRequest struct {
	Records []MixLogProducerApiRequestRecord `json:"records,omitempty"`
}

// MixLogProducerApiResponseRecord represents a single entry in the list of offsets returned in the Mix Log API Response
type MixLogProducerApiResponseRecord struct {
	Partition int    `json:"partition,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	ErrorCode int    `json:"error_code,omitempty"`
	Error     string `json:"error,omitempty"`
}

// MixLogProducerApiResponse represents a response object returned from Mix Log API
type MixLogProducerApiResponse struct {
	Offsets []MixLogProducerApiResponseRecord `json:"offsets,omitempty"`
}

// MixLogProducerApiClient manages the interaction with the Mix Log API storage
type MixLogProducerApiClient struct {
	httpclient.Http // mix log.api http client

	// resources for interacting with mix log api
	authenticator *oauth.Authenticator
	apiUrl        url.URL

	// thread-safe buffer of event log records
	records MixLogProducerApiRequestRecords

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

// NewStorage() instantiates a new instance of MixLogProducerApiClient
func NewStorage(config Config) wt.Storage {

	// parse mix log producer api url
	apiUrl, err := url.Parse(config.ApiUrl)
	if err != nil {
		log.Fatalf("invalid api url: %v", err)
	}

	// create instance of client
	s := &MixLogProducerApiClient{
		Http: httpclient.Http{
			HttpClient: http.Client{
				Timeout: time.Duration(time.Duration(config.ConnectTimeoutMs) * time.Millisecond),
			},
		},
		authenticator: oauth.NewAuthenticator(oauth.Credentials(config.Credentials)),
		apiUrl:        *apiUrl,
		job:           make(chan interface{}),
		record:        make(chan interface{}),
		done:          make(chan bool),

		ticker:      time.NewTicker(TICKER_TIMEOUT_MS * time.Millisecond),
		rateLimiter: ratelimiter.NewRateLimiter(config.RateLimit),
		maxRetries:  config.MaxRetries,
		delay:       config.Delay,
	}

	if _, err := s.authenticate(); err != nil {
		log.Fatalf("Invalid credentials provided: %v", err)
	}

	// set up worker pool
	numWorkers := DEFAULT_NUM_WORKERS
	if config.NumWorkers != 0 {
		numWorkers = config.NumWorkers
	}

	s.workers = workerpool.NewWorkerPool("mix-log-producer-api-workers", s.job, numWorkers, s.storageHandler)
	s.workers.Start()

	// run commit handler in background
	go s.commitHandler()

	return s
}

//// Helpers...

// storageHandler() is called by a workerpool worker to process a job
func (s *MixLogProducerApiClient) storageHandler(request interface{}) {
	var err error
	job := request.(*Job)
	switch job.Action {
	case WRITE:
		// We pass the record to the commitHandler() which handles batching and
		// writing of records to mix-log-producer-api
		start := time.Now()
		log.Debugf("writing to mix-log-producer-api record chan...")
		s.record <- request
		log.Debugf("done writing to mix-log-producer-api chan in %v ms...", time.Since(start).Milliseconds())
	case READ: // Read is not actually implemented...
		var data []byte
		data, err = s.Read(job.Index, job.DocID)
		log.Debugf("%v", string(data))
	case DELETE: // Delete is not actually implemented...
		err = s.Delete(job.Index, job.DocID)
	}

	if err != nil {
		log.Errorf("%v", err)
	}
}

// commitHandler() batches records and triggers a commit after max records have
// been added to the batch or after a timeout has been triggered
func (s *MixLogProducerApiClient) commitHandler() {
	lastCommit := time.Now()
	for {
		select {
		case record := <-s.record:
			if record != nil {
				// Add the record to the batch
				job := record.(*Job)
				var eventLog types.EventLogSchema
				json.Unmarshal(job.Body, &eventLog)

				s.records.Append(MixLogProducerApiRequestRecord{
					Key:   eventLog.Key,
					Value: eventLog.Value,
				})

				// If we've hit max record payload size, commit the batch to mix-log-producer-api
				if s.records.SizeInBytes() >= PAYLOAD_MAX_BYTES_THRESHOLD {
					log.Infof("mix log producer api bulk write triggered by record payload size [size: %.3f]", float64(s.records.SizeInBytes())/float64(1e+6))
					records := s.records.Records()
					go s.commit(records)
					s.records.Clear()
					lastCommit = time.Now()
				}
			}
		case <-s.ticker.C:
			log.Infof("mix-log-producer-api bulk write triggered by elapsed time")
			if s.records.Length() == 0 {
				log.Infof("no records to commit")
			} else if time.Since(lastCommit).Milliseconds() >= TICKER_TIMEOUT_MS {
				records := s.records.Records()
				go s.commit(records)
				s.records.Clear()
				lastCommit = time.Now()
			}
		case <-s.done:
			if s.records.Length() > 0 {
				records := s.records.Records()
				go s.commit(records)
				s.records.Clear()
			}
			s.commitDoneSignal.Wait()
			s.awaitShutdownSignal.Done()
		}
	}
}

// waitMyTurn() checks in with the rate limiter and waits if rate limit has been exceeded
func (s *MixLogProducerApiClient) waitMyTurn() {
	ctx := context.Background()
	startWait := time.Now()

	// rate limiting ...
	s.rateLimiter.Wait(ctx)
	log.Debugf("rate limited for %f seconds", time.Since(startWait).Seconds())
}

// randomSleep() generates a random delay between 0 - 60 seconds and sleeps for that duration
func (s *MixLogProducerApiClient) randomSleep() {
	// sleep a random number of seconds and then try again
	delay, _ := rand.Int(rand.Reader, big.NewInt(60))
	log.Infof("fetcher rate limited - sleeping %v seconds", delay)
	time.Sleep(time.Duration(delay.Add(delay, big.NewInt(1)).Int64()) * time.Second)
}

// monitorCommitResults() wraps logging and monitoring of the commit results
func (s *MixLogProducerApiClient) monitorCommitResults(start time.Time, numErrors, numIndexed int) {
	// monitor processing duration
	monitoring.SetGauge(wt.WriteDurationGauge,
		time.Since(start).Seconds(),
		s.apiUrl.String())

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

// processSuccessfulResponse() handles logging and monitoring when the api request is successful (may contain partial failures)
func (s *MixLogProducerApiClient) processSuccessfulResponse(responseBody []byte, numIndexed, numErrors *int) {
	var response MixLogProducerApiResponse
	err := json.Unmarshal(responseBody, &response)
	if err != nil {
		log.Errorf("error unmarshalling mix log producer api response: %v, %v", err, string(responseBody))
	} else {
		for _, offset := range response.Offsets {
			if offset.ErrorCode != 0 {
				log.Errorf("error committing record: %v %v", offset.ErrorCode, offset.Error)
				*numErrors += 1
				monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_MIX_LOG_PRODUCER_API, wt.NOT_WRITTEN, fmt.Sprintf("%v %v", offset.ErrorCode, offset.Error))
				monitoring.IncCounter(CommitTotalCounter, s.apiUrl.String(), fmt.Sprintf("%v %v", offset.ErrorCode, offset.Error))
			} else {
				*numIndexed += 1
				monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_MIX_LOG_PRODUCER_API, wt.WRITTEN, "")
				monitoring.IncCounter(CommitTotalCounter, s.apiUrl.String(), fmt.Sprintf("%v", offset.ErrorCode))
			}
		}
	}
}

// processedFailedResponse() handles retry, logging and monitoring when the api request fails
func (s *MixLogProducerApiClient) processedFailedResponse(retry int, statusCode int, err error) error {
	if s.maxRetries >= 0 && retry > s.maxRetries {
		monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_MIX_LOG_PRODUCER_API, wt.NOT_WRITTEN, fmt.Sprintf("%v %v", statusCode, err))
		monitoring.IncCounter(CommitTotalCounter, strconv.Itoa(statusCode), err.Error())
		return err
	}
	s.randomSleep()
	return nil
}

// processAuthorizationError() handles retry, logging and monitoring when the api request fails due to an authorization error
func (s *MixLogProducerApiClient) processAuthorizationError(retry int) error {
	if s.maxRetries >= 0 && retry > s.maxRetries {
		err := fmt.Errorf("authorization retry [%d] exceeded max retries [%d]", retry, s.maxRetries)
		monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_MIX_LOG_PRODUCER_API, wt.NOT_WRITTEN, err.Error())
		monitoring.IncCounter(CommitTotalCounter, strconv.Itoa(401), err.Error())
		return err
	}
	s.randomSleep()
	s.authenticator.GenerateToken()
	return nil
}

// commit() writes a batch of records to mix-log-producer-api
func (s *MixLogProducerApiClient) commit(records []MixLogProducerApiRequestRecord) error {
	s.commitDoneSignal.Add(1)
	defer s.commitDoneSignal.Done()

	// initialize some vars
	var start time.Time
	retry := 0
	numErrors := 0
	numIndexed := 0
	var err error

	// marshal records into bytes to be passed to mix log producer api
	data, _ := json.Marshal(MixLogProducerApiRequest{
		Records: records,
	})

	// call the mix-log-producer-api api, and then process the results
out:
	for {
		// check in with rate limiter...
		s.waitMyTurn()
		start = time.Now()

		// call the mix-log-producer-api endpoint
		var statusCode int
		var responseBody []byte
		log.Infof("writing %v records to mix log producer api", len(records))
		statusCode, responseBody, err = s.writeToLogProducerApi(data)
		log.Infof("log producer api status: %v %v", statusCode, err)

		switch {
		case statusCode >= 200 && statusCode < 300:
			// success
			s.processSuccessfulResponse(responseBody, &numIndexed, &numErrors)
			break out

		case statusCode == 401:
			// refresh token and try again
			retry++
			err = s.processAuthorizationError(retry)
			if err != nil {
				numErrors = len(records)
				break out
			}
		case err != nil || statusCode >= 300:
			// error. is it recoverable?
			retry++
			err = s.processedFailedResponse(retry, statusCode, err)
			if err != nil {
				numErrors = len(records)
				break out
			}
		}
	}
	s.monitorCommitResults(start, numErrors, numIndexed)
	return err
}

// writeToLogProducerApi() handles the post request to mix log producer api
func (s *MixLogProducerApiClient) writeToLogProducerApi(records []byte) (statusCode int, responseBody []byte, err error) {

	header := s.createHeader()
	if header == nil {
		return
	}

	// Observe write duration in seoncds and set histogram and guage metric
	timer := prometheus.NewTimer(CommitDurationHistogram)

	statusCode, responseBody, err = s.Post(&s.apiUrl, header, records)

	// monitor how long it takes to commit records to mix log producer api
	CommitDurationHistogram.Observe(timer.ObserveDuration().Seconds())

	return statusCode, responseBody, err
}

// authenticate() generates an oauth token to be used with the mix log producer api
func (s *MixLogProducerApiClient) authenticate() (*oauth.Token, error) {
	var err error
	token, err := s.authenticator.Authenticate()
	return token, err
}

// createheader() creates a valid request header to be used with the mix log producer api
func (s *MixLogProducerApiClient) createHeader() map[string][]string {
	token, err := s.authenticate()
	if err != nil {
		log.Errorf("error authenticating with mix log api: %v", err)
		return nil
	}

	header := map[string][]string{
		"Authorization": {fmt.Sprintf("%s %s", "Bearer", token.AccessToken)},
		"Content-Type":  {"application/json"},
	}
	return header
}

//// Interface methods

// Read() reads a record from mix-log-producer-api. Not implemented.
func (s *MixLogProducerApiClient) Read(key string, path string) ([]byte, error) {
	return nil, fmt.Errorf("not Implemented")
}

// Write() enqueues a record to be batch-written to mix-log-producer-api
func (s *MixLogProducerApiClient) Write(key string, path string, data []byte) (int, error) {
	// wrap record as a job and pass to worker pool
	s.job <- NewJob(WRITE, key, path, data)
	return len(data), nil
}

// Delete() deletes a record from mix-log-producer-api. Not implemented.
func (s *MixLogProducerApiClient) Delete(key string, path string) error {
	return fmt.Errorf("not Implemented")
}

// Shutdown() signals the mix-log-producer-api storage provider to shutdown
func (s *MixLogProducerApiClient) Shutdown() {
	s.awaitShutdownSignal.Add(1)
	s.workers.Stop()
	s.ticker.Stop()
	s.done <- true
	s.awaitShutdownSignal.Wait()
}
