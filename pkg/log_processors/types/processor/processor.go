package processor

/*
	Base processor implementation for custom processors to inherit from
*/

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
	c "nuance.xaas-logging.event-log-collector/pkg/cache"
	pt "nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io"
	iot "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/workerpool"
)

var (
	// Default base processor config
	DefaultLogProcessorYaml = LogProcessorYaml{
		Processor: LogProcessorParams{
			Name: "noop",
			Config: Config{
				NumWorkers: 1,
				FilterConfig: FilterConfig{
					Type:        "blacklist",
					ContentType: make([]string, 0),
				},
				CacheConfig: c.CacheConfig{
					Type:       "local",
					Expiration: 3600,
					Addr:       "redis:6379",
					DefaultTTL: 3600,
					DB:         0,
				},
			},
		},
	}
)

// The following structs represent a log processor yaml configuration structure
type LogProcessorYaml struct {
	Processor LogProcessorParams `json:"processor" yaml:"processor"`
}

type LogProcessorParams struct {
	Name   string `json:"name" yaml:"name"`
	Config Config `json:"config" yaml:"config"`
}

type FilterConfig struct {
	Type        string   `json:"type" yaml:"type"`
	ContentType []string `json:"content_type" yaml:"content_type"`
}

type Config struct {
	//PipelineWriter string        `json:"pipeline_writer" yaml:"pipeline_writer"`
	NumWorkers   int           `json:"num_workers" yaml:"num_workers"`
	FilterConfig FilterConfig  `json:"filter" yaml:"filter"`
	CacheConfig  c.CacheConfig `json:"cache" yaml:"cache"`
	Cache        c.Cache       `json:"dummy" yaml:"dummy"`
}

// LogProcessor provides a base LogProcessor implementation
type LogProcessor struct {
	Name string // the log processor name
	Cfg  Config // the log processor configation

	Records  chan []byte
	Job      chan interface{}
	Workers  *workerpool.WorkerPool
	Shutdown chan bool
	Wait     sync.WaitGroup

	Reader iot.Reader
	Writer iot.Writer

	WorkerFactory pt.WorkerFactory
	RunWorkerFunc pt.RunWorkerFunc
}

//// Public methods

// NewLogProcessor() creates a new instance of a base LogProcessor
func NewLogProcessor(configFile string) pt.LogProcessor {
	cfg, err := LoadConfigFromFile(configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}

	processor := LogProcessor{
		Name: cfg.Processor.Name,
		Cfg:  cfg.Processor.Config,

		Shutdown: make(chan bool),
		Records:  make(chan []byte, cfg.Processor.Config.NumWorkers),
		Job:      make(chan interface{}), // channel jobs to worker pool
	}

	// Initialize processor
	processor.Init(configFile, cfg)
	return &processor
}

// Init() wraps common initialization steps. It should be called while a processor is being created.
func (p *LogProcessor) Init(configFile string, cfg *LogProcessorYaml) {

	// Over-ride these two functions in your custom processor's NewLogProcessor()
	p.WorkerFactory = p.newWorker
	p.RunWorkerFunc = p.runWorker

	// Initialize the worker pool to fan-out processing of event logs
	p.Workers = workerpool.NewWorkerPool(cfg.Processor.Name, p.Job, cfg.Processor.Config.NumWorkers, p.jobHandler)
	p.Workers.Start()
	p.Wait.Add(1)

	var err error
	// Create a cache that workers can use to store event logs if necessary
	p.Cfg.Cache, err = c.CreateCache(p.Cfg.CacheConfig)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Create data pipeline reader and writer
	p.Reader, err = io.CreateReader(pipeline.PROCESSOR, configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}
	p.Writer, err = io.CreateWriter(pipeline.PROCESSOR, configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}
}

//// Helper methods

// LoadConfigFromFile() loads the processor yaml config from file. Over-rides config file settings with env vars if set.
func LoadConfigFromFile(configFile string) (*LogProcessorYaml, error) {
	// Start with defaults
	yml := DefaultLogProcessorYaml

	// Read config file content
	file, err := os.ReadFile(filepath.Clean(configFile))
	if err == nil {
		// Unmarshall yaml
		err = yaml.Unmarshal(file, &yml)
		if err != nil {
			return nil, err
		}
	}

	raw, _ := json.Marshal(log.MaskSensitiveData(yml))
	log.Debugf("%v", string(raw))

	return &yml, nil
}

//// Private methods

// fetchRecords() reads event logs from the data pipeline
func (p *LogProcessor) fetchRecords() error {

	for {
		select {
		case <-p.Shutdown:
			log.Debugf("processor has stopped fetching records")
			p.Wait.Done()
			return nil
		default:
			log.Debugf("processor reading record...")
			record, err := p.Reader.Read()
			if err != nil {
				if err.Error() != "queue is empty" {
					log.Errorf("%v", err)
				}
				time.Sleep(5 * time.Second)
			} else {
				log.Debugf("processor passing record to worker pool")
				p.Job <- record
			}
		}
	}
}

// newWorker() is the factory method for creating a base worker. Over-ride this method to create
// customized workers in processors that extend base processor
func (p *LogProcessor) newWorker() pt.LogProcessorWorker {
	log.Debugf("creating a base processor worker")
	return NewWorker(p.Cfg)
}

// runWorker() implements the desired process chaining for a given processor.
// For example: worker.Start(record).Validate().Filter().Join().Transform().Writer(p.Writer)
func (p *LogProcessor) runWorker(worker pt.LogProcessorWorker, record []byte) {
	// Just validate and then write the event log to the data pipeline.
	worker.Start(record).Validate().Write(p.Writer)
}

// jobHandler() is called by the worker pool. It creates, runs, and monitors the work of processing a record.
func (p *LogProcessor) jobHandler(job interface{}) {
	record := job.([]byte)

	start := time.Now() // get processing start time
	topic := "unknown"
	var dt time.Time

	// Create a new worker to process the event log and then run the worker
	worker := p.WorkerFactory()
	p.RunWorkerFunc(worker, record)

	// If event log is valid, monitor lag in
	if worker.IsValid() {
		topic = worker.EventLog().Topic
		dt, _ = time.Parse(time.RFC3339, worker.EventLog().Value.Timestamp)
		monitoring.SetGauge(monitoring.ElcLagIn, float64(time.Since(dt).Milliseconds()),
			monitoring.PROM_LABEL_ELC_COMPONENT_PROCESSOR,
			topic)
	}

	// monitor records in counter
	monitoring.IncCounter(monitoring.ElcNumRecordsInCounter,
		monitoring.PROM_LABEL_ELC_COMPONENT_PROCESSOR,
		topic)

	// monitor lag out and records out counter
	status := monitoring.PROM_STATUS_FAILED
	if worker.IsWritten() {
		status = monitoring.PROM_STATUS_SUCCESS
		monitoring.SetGauge(monitoring.ElcLagOut, float64(time.Since(dt).Milliseconds()),
			monitoring.PROM_LABEL_ELC_COMPONENT_PROCESSOR,
			topic)

		monitoring.IncCounter(monitoring.ElcNumRecordsOutCounter,
			monitoring.PROM_LABEL_ELC_COMPONENT_PROCESSOR,
			topic)
	}

	// monitor total logs processed broken down by disposition
	message := monitoring.PROM_MSG_PROCESSED
	code := http.StatusOK
	switch {
	case worker.IsFiltered():
		message = monitoring.PROM_MSG_FILTERED
	case worker.IsFailed():
		message = worker.LastError().Error()
		code = monitoring.PROM_ERR_PROCESS_ERROR
		monitoring.IncCounter(monitoring.ElcErrors,
			monitoring.PROM_LABEL_ELC_COMPONENT_PROCESSOR,
			topic,
			fmt.Sprint(monitoring.PROM_ERR_PIPELINE_WRITE_ERROR),
			message)
	default:
		// do nothing
	}

	// monitor processing duration
	monitoring.SetGauge(monitoring.ElcProcessingDuration, float64(time.Since(start).Milliseconds()),
		monitoring.PROM_LABEL_ELC_COMPONENT_PROCESSOR,
		topic,
		status,
		fmt.Sprint(code),
		message)

	// monitor total records processed
	monitoring.IncCounter(monitoring.ElcProcessingTotal,
		monitoring.PROM_LABEL_ELC_COMPONENT_PROCESSOR,
		topic,
		status,
		fmt.Sprint(code),
		message)
}

//// Interface methods

// Start() implements the LogProcessor interface. It starts async fetching of event log records available
// in the data pipeline and processes them before writing them back into the data pipeline
func (p *LogProcessor) Start() error {
	log.Infof("starting %v processer", p.Name)

	err := p.Reader.Open()
	if err != nil {
		return err
	}

	err = p.Writer.Open()
	if err != nil {
		return err
	}

	go p.fetchRecords()

	return nil
}

// Stop() implements the LogProcessor interface. It shuts down event log processing
func (p *LogProcessor) Stop() {
	log.Debugf("stopping log processor")
	p.Shutdown <- true
	log.Debugf("stopping workers..")
	p.Workers.Stop()
	log.Debugf("processor stopped")
}

// GetStatus() implements the LogProcessor interface. Currently not doing much...
func (p *LogProcessor) GetStatus() (pt.LogProcessorStatus, error) {
	return pt.LogProcessorStatus{}, nil
}
