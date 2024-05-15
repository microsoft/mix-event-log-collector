package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/ratelimiter"
	t "nuance.xaas-logging.event-log-collector/pkg/types"

	"nuance.xaas-logging.event-log-collector/pkg/workerpool"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

const (
	REMOTE_SERVICE_MONGODB = "MongoDB"
)

type MongoDBRecord struct {
	t.EventLogSchema
	ID string `bson:"_id,omitempty"`
}

type MongoDBClient struct {
	client   *mongo.Client
	endpoint string

	collection *mongo.Collection
	options    *options.FindOneAndReplaceOptions

	commitDoneSignal    sync.WaitGroup
	awaitShutdownSignal sync.WaitGroup
	done                chan bool

	workers *workerpool.WorkerPool
	job     chan interface{}
	record  chan interface{}

	rateLimiter *rate.Limiter
	maxRetries  int
	delay       int
}

//// Public methods

func NewStorage(config Config) wt.Storage {

	storage := &MongoDBClient{
		endpoint: config.MongoDBUri,

		job:    make(chan interface{}),
		record: make(chan interface{}),
		done:   make(chan bool),

		rateLimiter: ratelimiter.NewRateLimiter(config.RateLimit),
		maxRetries:  config.MaxRetries,
		delay:       config.Delay,
	}

	client, err := storage.createMongoDbClient()
	if err != nil {
		log.Fatalf("%v", err)
	}
	storage.client = client
	storage.collection = storage.client.Database(config.DatabaseName).Collection(config.CollectionName)
	storage.options = options.FindOneAndReplace().SetUpsert(true)

	numWorkers := DEFAULT_NUM_WORKERS
	if config.NumWorkers != 0 {
		numWorkers = config.NumWorkers
	}

	storage.workers = workerpool.NewWorkerPool("mongodb-workers", storage.job, numWorkers, storage.storageHandler)
	storage.workers.Start()
	go storage.commitHandler()

	return storage
}

//// Helpers...

func (m *MongoDBClient) createMongoDbClient() (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	clientOptions := options.Client().ApplyURI(m.endpoint).SetDirect(true)
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to create mongo client: %v", err)
	}

	err = client.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize connection with mongodb: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Errorf("ping failed to connect to mongodb: %v", err)
	}

	return client, nil
}

// storageHandler() is called by a workerpool worker to process a job
func (m *MongoDBClient) storageHandler(request interface{}) {
	var err error
	job := request.(*Job)
	switch job.Action {
	case WRITE:
		// We pass the record to the commitHandler() which handles writing of records to mongodb
		start := time.Now()
		log.Debugf("writing to mongodb record chan...")
		m.record <- request
		log.Debugf("done writing to mongodb chan in %v ms...", time.Since(start).Milliseconds())
	case READ: // Read is not actually implemented...
		var data []byte
		data, err = m.Read(job.Index, job.DocID)
		log.Debugf("%v", string(data))
	case DELETE: // Delete is not actually implemented...
		err = m.Delete(job.Index, job.DocID)

	}
	if err != nil {
		log.Errorf("%v", err)
	}
}

// commitHandler() batches records and triggers a commit after max records have
// been added to the batch or after a timeout has been triggered
func (m *MongoDBClient) commitHandler() {
	for {
		select {
		case record := <-m.record:
			if record != nil {
				// Commit the record
				job := record.(*Job)
				var doc MongoDBRecord
				err := json.Unmarshal(job.Body, &doc)
				if err != nil {
					log.Errorf("%v", err)
					monitoring.IncCounter(CommitTotalCounter, m.collection.Name(), err.Error())
					continue
				}
				doc.ID = job.Index
				go m.commit(doc)
			}
		case <-m.done:
			m.commitDoneSignal.Wait()
			m.awaitShutdownSignal.Done()
		}
	}
}

func (m *MongoDBClient) waitMyTurn() {
	ctx := context.Background()
	startWait := time.Now()

	// rate limiting ...
	m.rateLimiter.Wait(ctx)
	log.Debugf("rate limited for %f seconds", time.Since(startWait).Seconds())
}

func (m *MongoDBClient) monitorCommitResults(start time.Time, err error) {
	// monitor processing duration
	monitoring.SetGauge(wt.WriteDurationGauge,
		time.Since(start).Seconds(),
		m.collection.Name())

	dur := time.Since(start).Milliseconds()
	if err != nil {
		monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_MONGODB, wt.NOT_WRITTEN, err.Error())
		monitoring.IncCounter(CommitTotalCounter, m.collection.Name(), err.Error())

		log.Errorf(
			"error indexing document in %v ms (%d docs/sec): %v",
			dur,
			int64((1000.0 / float64(dur))),
			err,
		)
	} else {
		monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_MONGODB, wt.WRITTEN, "")
		monitoring.IncCounter(CommitTotalCounter, m.collection.Name(), monitoring.PROM_STATUS_SUCCESS)

		log.Infof(
			"Indexed document in %v ms (%d docs/sec)",
			dur,
			int64((1000.0 / float64(dur))),
		)
	}
}

func (m *MongoDBClient) processNetworkFailure(retry int) error {
	if m.maxRetries >= 0 && retry > m.maxRetries {
		err := fmt.Errorf("retry [%d] exceeded max retries [%d]", retry, m.maxRetries)
		log.Errorf("network error: %s", err.Error())
		return err
	}
	time.Sleep(time.Duration(time.Duration(m.delay) * time.Second))
	return nil
}

func (m *MongoDBClient) upsertMongoDBDoc(doc MongoDBRecord) error {
	filter := bson.D{{Key: "_id", Value: doc.ID}}
	var replacedDocument bson.M
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()

	// Observe write duration in seoncds and set histogram and guage metric
	timer := prometheus.NewTimer(CommitDurationHistogram)

	err := m.collection.FindOneAndReplace(
		context.TODO(),
		filter,
		doc,
		m.options,
	).Decode(&replacedDocument)

	// monitor how long it takes to commit records to mongodb
	CommitDurationHistogram.Observe(timer.ObserveDuration().Seconds())

	switch err {
	case mongo.ErrNoDocuments:
		log.Debugf("inserted new record [id = %v]", doc.ID)
		return nil
	case nil:
		log.Debugf("updated existing record [id = %v]", doc.ID)
		return nil
	default:
		log.Errorf("find and replace record failed: %v", err)
		return err
	}
}

// commit() writes a batch of records to mongodb
func (m *MongoDBClient) commit(record MongoDBRecord) {
	m.commitDoneSignal.Add(1)
	defer m.commitDoneSignal.Done()

	var start time.Time
	var err error
	retry := 0

	// call the mongodb Bulk() api, and then process the results
	for {
		// check in with rate limiter...
		m.waitMyTurn()
		start = time.Now()

		// call the mongodb api
		err = m.upsertMongoDBDoc(record)
		if err != nil {
			retry++
			if m.processNetworkFailure(retry) == nil {
				continue
			}
		}

		break
	}
	m.monitorCommitResults(start, err)
}

// Read() reads a record from mongodb. Not implemented.
func (m *MongoDBClient) Read(key string, path string) ([]byte, error) {
	return nil, fmt.Errorf("not Implemented")
}

// Write() enqueues a record to be batch-written to mongodb
func (m *MongoDBClient) Write(key string, path string, data []byte) (int, error) {
	// wrap record as a job and pass to worker pool
	m.job <- NewJob(WRITE, key, path, data)
	return len(data), nil
}

// Delete() deletes a record from mongodb. Not implemented.
func (m *MongoDBClient) Delete(key string, path string) error {
	return fmt.Errorf("not Implemented")
}

// Shutdown() signals the mongodb storage provider to shutdown
func (m *MongoDBClient) Shutdown() {
	m.awaitShutdownSignal.Add(1)
	m.workers.Stop()
	m.done <- true
	m.awaitShutdownSignal.Wait()
}
