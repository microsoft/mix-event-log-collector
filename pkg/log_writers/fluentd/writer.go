package fluentd

/*
 * fluentd writer implements a log writer that uses fluentd's in_forward
 * TCP socket input plugin as the event log storage provider. Leverage the fluentd log writer
 * for writing to storage providers not currently supported directly within event log collector
 */

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io"
	iot "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/types"
)

var (
	// Default fluentd writer config
	DefaultLogWriterYaml = LogWriterYaml{
		Writer: LogWriterParams{
			Name: "fluentd",
			Config: Config{
				NumWorkers:  5,
				Address:     "127.0.0.1:24224",
				Buffered:    false,
				BufferLimit: 8 * 1024 * 1024,
			},
		},
	}
)

// The following structs represent a log writer yaml configuration structure
type LogWriterYaml struct {
	Writer LogWriterParams `json:"writer" yaml:"writer"`
}

type LogWriterParams struct {
	Name   string `json:"name" yaml:"name"`
	Config Config `json:"config" yaml:"config"`
}

type Config struct {
	NumWorkers  int    `json:"num_workers" yaml:"num_workers"`
	Address     string `json:"address" yaml:"address"`
	Tag         string `json:"tag" yaml:"tag"`
	Buffered    bool   `json:"buffered" yaml:"buffered"`
	BufferLimit int    `json:"buffer_limit" yaml:"buffer_limit"`
}

// LogWriter is the fluentd LogWriter implementation
type LogWriter struct {
	name string
	cfg  Config

	records chan []byte

	// signaling
	shutdown         chan bool
	jobHandlerSignal chan bool
	awaitShutdown    sync.WaitGroup

	reader  iot.Reader
	storage wt.Storage
}

//// Public methods

// NewLogWriter() creates a new instance of a fluentd LogWriter
func NewLogWriter(configFile string) wt.LogWriter {
	log.Debugf("factory creating new fluentd log writer")

	cfg, err := loadConfigFromFile(configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}

	writer := LogWriter{
		name:             cfg.Writer.Name,
		cfg:              cfg.Writer.Config,
		shutdown:         make(chan bool),
		jobHandlerSignal: make(chan bool),
		records:          make(chan []byte, cfg.Writer.Config.NumWorkers),
		storage:          NewStorage(cfg.Writer.Config.Address, cfg.Writer.Config.Buffered, cfg.Writer.Config.BufferLimit),
	}
	writer.reader, err = io.CreateReader(pipeline.WRITER, configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}

	return &writer
}

//// Helper methods

// loadConfigFromFile() loads the writer yaml config from file
func loadConfigFromFile(configFile string) (*LogWriterYaml, error) {
	// Start with defaults
	yml := DefaultLogWriterYaml

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

// getIndex() returns the index value to be used for the event log. For fluentd, this
// will map to the tag in the fluent.conf file. The value to use for index can be optionally
// set in the event log collector config yaml, or default to the Topic (ie Mix App ID) in the event log
func (l *LogWriter) getIndex(eventLog types.EventLogSchema) string {
	if len(l.cfg.Tag) == 0 {
		return eventLog.Topic
	}
	return l.cfg.Tag
}

//// Private methods

// fetchRecords() reads event logs from the data pipeline
func (l *LogWriter) fetchRecords() error {

	for {
		select {
		case <-l.shutdown:
			l.reader.Close()
			log.Debugf("log writer has stopped fetching records")
			l.jobHandlerSignal <- true
			return nil
		default:
			record, err := l.reader.Read()
			if err != nil {
				if err.Error() != "queue is empty" {
					log.Errorf("%v", err)
				}
				time.Sleep(5 * time.Second)
			} else {
				log.Debugf("writer passing record to storage provider")
				l.records <- record
			}
		}
	}
}

// jobHandler() processes records as they're received. It provides monitoring of the write to storage process.
func (l *LogWriter) jobHandler() {
	monitor := func(err error, start time.Time, topic string, eventLogCreated time.Time) {
		// monitor processing duration
		status := monitoring.PROM_STATUS_SUCCESS
		message := monitoring.PROM_MSG_PROCESSED
		code := http.StatusOK

		if err != nil {
			status = monitoring.PROM_STATUS_FAILED
			message = err.Error()
			code = monitoring.PROM_ERR_PROCESS_ERROR

			monitoring.SetGauge(monitoring.ElcProcessingDuration, float64(time.Since(start).Milliseconds()),
				monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
				topic,
				monitoring.PROM_STATUS_FAILED,
				fmt.Sprint(monitoring.PROM_ERR_PIPELINE_WRITE_ERROR),
				message)

			monitoring.IncCounter(monitoring.ElcErrors,
				monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
				topic,
				fmt.Sprint(monitoring.PROM_ERR_PIPELINE_WRITE_ERROR),
				message)
		} else {
			monitoring.SetGauge(monitoring.ElcProcessingDuration, float64(time.Since(start).Milliseconds()),
				monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
				topic,
				monitoring.PROM_STATUS_SUCCESS,
				fmt.Sprint(code),
				message)

			monitoring.SetGauge(monitoring.ElcLagOut, float64(time.Since(eventLogCreated).Milliseconds()),
				monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
				topic)

			monitoring.IncCounter(monitoring.ElcNumRecordsOutCounter,
				monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
				topic)
		}

		monitoring.IncCounter(monitoring.ElcProcessingTotal,
			monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
			topic,
			status,
			fmt.Sprint(code),
			message)
	}

	var processingComplete sync.WaitGroup
	for {
		select {
		case <-l.jobHandlerSignal:
			processingComplete.Wait()
			l.storage.Shutdown()
			log.Infof("log writer has been stopped")
			l.awaitShutdown.Done()
		case record := <-l.records:
			processingComplete.Add(1)

			log.Debugf("storage provider processing %d bytes", len(record))
			start := time.Now()

			// convert bytes to event log
			topic := "unknown"
			var dt time.Time
			var eventLog types.EventLogSchema
			err := json.Unmarshal(record, &eventLog)
			if err == nil {
				topic = eventLog.Topic
				dt, _ = time.Parse(time.RFC3339, eventLog.Value.Timestamp)
				index := l.getIndex(eventLog)
				docID := fmt.Sprintf("%s.%s.%v.%s",
					eventLog.Value.ID,
					eventLog.Topic,
					eventLog.Partition,
					strconv.FormatFloat(eventLog.Offset, 'f', 0, 64))

				monitoring.IncCounter(monitoring.ElcNumRecordsInCounter,
					monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
					topic)
				monitoring.SetGauge(monitoring.ElcLagIn, float64(time.Since(dt).Milliseconds()),
					monitoring.PROM_LABEL_ELC_COMPONENT_WRITER,
					topic)

				_, err = l.storage.Write(index, fmt.Sprintf("%v.json", docID), record)
				if err != nil {
					log.Errorf("error writing document %vf: %v", docID, err)
				}
			}
			monitor(err, start, topic, dt)
			processingComplete.Done()
		}
	}
}

//// Interface methods

// Start() implements the LogWriter interface. It starts async reading of event log records available
func (l *LogWriter) Start() error {
	err := l.reader.Open()
	if err != nil {
		return err
	}

	go l.jobHandler()
	go l.fetchRecords()

	log.Infof("Starting %v writer. Writing logs to %v", l.name, l.cfg.Address)
	return nil
}

// Stop() implements the LogWriter interface. It shuts down event log writing
func (l *LogWriter) Stop() {
	l.awaitShutdown.Add(1)
	l.shutdown <- true
	l.awaitShutdown.Wait()
}

// GetStatus() implements the LogWriter interface. Currently not doing much...
func (l *LogWriter) GetStatus() (wt.LogWriterStatus, error) {
	return wt.LogWriterStatus{}, nil
}
