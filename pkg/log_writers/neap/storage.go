package neap

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/ratelimiter"

	"nuance.xaas-logging.event-log-collector/pkg/workerpool"

	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type Compression int

const (
	TICKER_TIMEOUT_MS = 30000

	GZIP Compression = iota
	ZIP
)

// NeapRecords provides a thread-safe implementation of a buffer of neap json records
type NeapRecords struct {
	data       bytes.Buffer
	numRecords int
	mu         sync.Mutex
}

// Length() returns the number of items in the records buffer
func (r *NeapRecords) Length() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	length := r.numRecords
	return length
}

// SizeInBytes() returns the size in bytes of the records buffer
func (r *NeapRecords) SizeInBytes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := r.data.Len()
	return size
}

// Append() adds an event log record to the records buffer
func (r *NeapRecords) Append(value []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data := append(value, "\n"...)

	r.data.Grow(len(data))
	r.data.Write(data)

	r.numRecords++
}

// Records() returns a copy of the records buffer
func (r *NeapRecords) Records() bytes.Buffer {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Len()-1 ensures we remove trailing new line
	records := make([]byte, r.data.Len()-1)
	r.data.Read(records)
	return *bytes.NewBuffer(records)
}

// Clear() removes all entries from the records buffer
func (r *NeapRecords) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data.Reset()
	r.numRecords = 0
}

// NeapClient manages the writing of data to neap storage
type NeapClient struct {
	path                 string
	compression          Compression
	recordCountThreshold int

	// thread-safe buffer of event log records
	records NeapRecords

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
}

//// Public methods

// NewStorage() instantiates a new instance of NeapClient
func NewStorage(config Config) wt.Storage {

	// create instance of client
	nc := &NeapClient{
		path:                 config.Path,
		recordCountThreshold: config.MaxRecords,
		job:                  make(chan interface{}),
		record:               make(chan interface{}),
		done:                 make(chan bool),

		ticker:      time.NewTicker(TICKER_TIMEOUT_MS * time.Millisecond),
		rateLimiter: ratelimiter.NewRateLimiter(config.RateLimit),
	}

	switch config.Compression {
	case "zip":
		nc.compression = ZIP
	default:
		nc.compression = GZIP
	}

	// set up worker pool
	numWorkers := DEFAULT_NUM_WORKERS
	if config.NumWorkers != 0 {
		numWorkers = config.NumWorkers
	}

	nc.workers = workerpool.NewWorkerPool("neap-workers", nc.job, numWorkers, nc.storageHandler)
	nc.workers.Start()

	// run commit handler in background
	go nc.commitHandler()

	return nc
}

//// Helpers...

// storageHandler() is called by a workerpool worker to process a job
func (n *NeapClient) storageHandler(request interface{}) {
	var err error
	job := request.(*Job)
	switch job.Action {
	case WRITE:
		// We pass the record to the commitHandler() which handles batching and
		// writing of records to neap
		start := time.Now()
		log.Debugf("writing to neap record chan...")
		n.record <- request
		log.Debugf("done writing to neap chan in %v ms...", time.Since(start).Milliseconds())
	case READ: // Read is not actually implemented...
		var data []byte
		data, err = n.Read(job.Index, job.DocID)
		log.Debugf("%v", string(data))
	case DELETE: // Delete is not actually implemented...
		err = n.Delete(job.Index, job.DocID)

	}
	if err != nil {
		log.Errorf("%v", err)
	}
}

// commitHandler() batches records and triggers a commit after max records have
// been added to the batch or after a timeout has been triggered
func (n *NeapClient) commitHandler() {
	lastCommit := time.Now()
	for {
		select {
		case record := <-n.record:
			if record != nil {
				// Add the record to the batch
				job := record.(*Job)

				n.records.Append(job.Body)

				// If we've hit max batch record size, commit the batch to neap
				if n.records.Length() >= n.recordCountThreshold {
					log.Infof("neap bulk write triggered by record count")
					log.Infof("committing batch [count: %v, queue: %v]", n.records.Length(), len(n.record))
					go n.commit(n.records.Records(), n.records.Length())
					n.records.Clear()
					lastCommit = time.Now()
				}
			}
		case <-n.ticker.C:
			log.Infof("neap bulk write triggered by elapsed time")
			if n.records.Length() == 0 {
				log.Infof("no records to commit")
			} else if time.Since(lastCommit).Milliseconds() >= TICKER_TIMEOUT_MS {
				go n.commit(n.records.Records(), n.records.Length())
				n.records.Clear()
				lastCommit = time.Now()
			}
		case <-n.done:
			if n.records.Length() > 0 {
				go n.commit(n.records.Records(), n.records.Length())
				n.records.Clear()
			}
			n.commitDoneSignal.Wait()
			n.awaitShutdownSignal.Done()
		}
	}
}

// waitMyTurn() checks in with the rate limiter and waits if rate limit has been exceeded
func (n *NeapClient) waitMyTurn() {
	ctx := context.Background()
	startWait := time.Now()

	// rate limiting ...
	n.rateLimiter.Wait(ctx)
	log.Debugf("rate limited for %f seconds", time.Since(startWait).Seconds())
}

// monitorCommitResults() wraps logging and monitoring of the commit results
func (n *NeapClient) monitorCommitResults(start time.Time, numErrors, numIndexed int) {
	// monitor processing duration
	monitoring.SetGauge(wt.WriteDurationGauge,
		time.Since(start).Seconds(),
		n.path)

	dur := time.Since(start).Milliseconds()
	if numErrors > 0 {
		log.Errorf(
			"Failed to save [%v] records in %v ms (%d docs/sec)",
			numErrors,
			dur,
			int64((1000.0/float64(dur))*float64(numErrors)),
		)
	} else if numIndexed > 0 {
		log.Infof(
			"Successfully saved [%v] documents in %v ms (%d docs/sec)",
			numIndexed,
			dur,
			int64((1000.0/float64(dur))*float64(numIndexed)),
		)
	} else {
		log.Infof("numErrors: %d, numIndexed: %d", numErrors, numIndexed)
	}
}

// processBulkResponseCompleteFailure() handles logging and monitoring when the neap bulk api request fails completely
func (n *NeapClient) processError(err error, numErrors int) {
	monitoring.AddCounter(wt.WritesTotalCounter, float64(numErrors), wt.LABEL_NEAP, wt.NOT_WRITTEN, err.Error())
	monitoring.AddCounter(CommitTotalCounter, float64(numErrors), fmt.Sprintf("%v", err), err.Error())
}

func (n *NeapClient) processSuccess(count int) {
	monitoring.AddCounter(wt.WritesTotalCounter, float64(count), wt.LABEL_NEAP, wt.WRITTEN, "")
	monitoring.AddCounter(CommitTotalCounter, float64(count), "200", "OK")
}

func (n *NeapClient) createZipFile(path string, records []byte) error {

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	t := time.Now().UTC()
	tfmt := fmt.Sprintf("%v_%02v%02v%02v_%03v", t.Format("01022006"), t.Hour(), t.Minute(), t.Second(), t.UnixMilli())
	filename := fmt.Sprintf("records_%v.json", tfmt)
	zipFile, err := zipWriter.Create(filename)
	if err != nil {
		return err
	}

	_, err = zipFile.Write(records)
	if err != nil {
		return err
	}

	// 2. write zip to file
	// Make sure to check the error on Close.
	err = zipWriter.Close()
	if err != nil {
		return err
	}

	//write the zipped file to the disk
	err = os.WriteFile(fmt.Sprintf("%v/records_%v.zip", path, tfmt), buf.Bytes(), 0600)
	if err != nil {
		return err
	}

	return nil
}

func (n *NeapClient) createGzipFile(path string, records []byte) error {
	t := time.Now().UTC()
	tfmt := fmt.Sprintf("%v_%02v%02v%02v_%03v", t.Format("01022006"), t.Hour(), t.Minute(), t.Second(), t.UnixMilli())

	// Create output file
	out, err := os.Create(fmt.Sprintf("%v/records_%v.gz", path, tfmt))
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	_, err = gw.Write(records)
	return err
}

// commit() writes a batch of records to neap
func (n *NeapClient) commit(records bytes.Buffer, count int) (int, error) {
	n.commitDoneSignal.Add(1)
	defer n.commitDoneSignal.Done()

	start := time.Now()
	numErrors := 0
	numIndexed := 0
	var err error

	// call the neap Bulk() api, and then process the results
	// check in with rate limiter...
	n.waitMyTurn()

	timer := prometheus.NewTimer(CommitDurationHistogram)
	defer CommitDurationHistogram.Observe(timer.ObserveDuration().Seconds())

	path := fmt.Sprintf("%v/%v", n.path, time.Now().UTC().Format("01-02-2006"))
	if err := os.MkdirAll(path, 0750); err != nil {
		n.processError(err, count)
		numErrors = count
		n.monitorCommitResults(start, numErrors, numIndexed)
		return numIndexed, err
	}

	switch n.compression {
	case ZIP:
		err = n.createZipFile(path, records.Bytes())
	default:
		err = n.createGzipFile(path, records.Bytes())
	}

	if err != nil {
		n.processError(err, count)
		numErrors = count
	} else {
		n.processSuccess(count)
		numIndexed = count
	}

	n.monitorCommitResults(start, numErrors, numIndexed)
	return numIndexed, err
}

// Read() reads a record from neap. Not implemented.
func (n *NeapClient) Read(key string, path string) ([]byte, error) {
	return nil, fmt.Errorf("not Implemented")
}

// Write() enqueues a record to be batch-written to neap
func (n *NeapClient) Write(key string, path string, data []byte) (int, error) {
	// wrap record as a job and pass to worker pool
	n.job <- NewJob(WRITE, key, path, data)
	return len(data), nil
}

// Delete() deletes a record from neap. Not implemented.
func (n *NeapClient) Delete(key string, path string) error {
	return fmt.Errorf("not Implemented")
}

// Shutdown() signals the neap storage provider to shutdown
func (n *NeapClient) Shutdown() {
	n.awaitShutdownSignal.Add(1)
	n.workers.Stop()
	n.ticker.Stop()
	n.done <- true
	n.awaitShutdownSignal.Wait()
}
