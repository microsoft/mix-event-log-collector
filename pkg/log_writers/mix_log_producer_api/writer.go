package mix_log_producer_api

/*
 * mix-log-producer-api writer implements a log writer that writes event logs to
 * mix-log-producer-api as the storage provider.
 */

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io"
	iot "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/ratelimiter"
	t "nuance.xaas-logging.event-log-collector/pkg/types"
)

var (
	// Default mix-log-producer-api writer config
	DefaultLogWriterYaml = LogWriterYaml{
		Writer: LogWriterParams{
			Name: "mix-log-producer-api",
			Config: Config{
				Credentials: Credentials{
					AuthDisabled:  false,
					TokenURL:      "https://auth.crt.nuance.com/oauth2/token",
					Scope:         "log.write",
					AuthTimeoutMS: 5000,
				},
				ApiUrl:     "https://log.api.nuance.com/producers",
				NumWorkers: 5,
				RateLimit: ratelimiter.RateLimit{
					Limit: 10,
					Burst: 1,
				},
				MaxRetries: 10,
				Delay:      10,
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

type Credentials struct {
	AuthDisabled  bool   `json:"auth_disabled" yaml:"auth_disabled"`
	TokenURL      string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	ClientID      string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	ClientSecret  string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
	Scope         string `json:"scope,omitempty" yaml:"scope,omitempty"`
	AuthTimeoutMS int    `json:"auth_timeout_ms,omitempty" yaml:"auth_timeout_ms,omitempty"`
}

type Config struct {
	Credentials      Credentials           `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	ApiUrl           string                `json:"mix_log_producer_api_url,omitempty" yaml:"mix_log_producer_api_url,omitempty"`
	ConnectTimeoutMs int                   `json:"connect_timeout_ms,omitempty" yaml:"connect_timeout_ms,omitempty"`
	RateLimit        ratelimiter.RateLimit `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty"`
	MaxRetries       int                   `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
	Delay            int                   `json:"retry_delay,omitempty" yaml:"retry_delay,omitempty"`
	NumWorkers       int                   `json:"num_workers" yaml:"num_workers"`
}

// LogWriter is the mix-log-producer-api LogWriter implementation
type LogWriter struct {
	name string
	cfg  Config

	records chan []byte

	// signaling
	shutdown         chan bool
	jobHandlerSignal chan bool
	awaitShutdown    sync.WaitGroup

	reader      iot.Reader
	logProducer wt.Storage
}

//// Public methods

// NewLogWriter() creates a new instance of an mix-log-producer-api LogWriter
func NewLogWriter(configFile string) wt.LogWriter {
	log.Infof("factory creating new mix-log-producer-api log writer")

	cfg, err := loadConfigFromFile(configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}

	if len(strings.Trim(cfg.Writer.Config.Credentials.ClientID, " ")) == 0 {
		log.Fatalf("Invalid client_id provided")
	}

	if len(strings.Trim(cfg.Writer.Config.Credentials.ClientSecret, " ")) == 0 {
		log.Fatalf("Invalid client_secret provided")
	}

	writer := LogWriter{
		name:             cfg.Writer.Name,
		cfg:              cfg.Writer.Config,
		shutdown:         make(chan bool),
		jobHandlerSignal: make(chan bool),
		records:          make(chan []byte, cfg.Writer.Config.NumWorkers),
		logProducer:      NewStorage(cfg.Writer.Config),
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

	// Over-ride some fetcher config values with env vars if they've been set
	yml.Writer.Config.Credentials.ClientID = getValueFromEnv("ELC_CLIENT_ID", yml.Writer.Config.Credentials.ClientID)
	yml.Writer.Config.Credentials.ClientSecret = getValueFromEnv("ELC_CLIENT_SECRET", yml.Writer.Config.Credentials.ClientSecret)
	yml.Writer.Config.Credentials.TokenURL = getValueFromEnv("ELC_TOKEN_URL", yml.Writer.Config.Credentials.TokenURL)
	yml.Writer.Config.ApiUrl = getValueFromEnv("ELC_MIX_LOG_PRODUCER_API_URL", yml.Writer.Config.ApiUrl)

	raw, _ := json.Marshal(log.MaskSensitiveData(yml))
	log.Debugf("%v", string(raw))

	return &yml, nil
}

// getValueFromEnv() returns the value of the env var if available. If not available, returns the default value
func getValueFromEnv(name string, defaultVal string) string {
	if val, found := os.LookupEnv(name); found {
		return val
	}
	return defaultVal
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
				switch err.Error() {
				case "reader closed":
					log.Debugf("%v", err)
				case "queue is empty":
					time.Sleep(5 * time.Second)
				default:
					log.Errorf("%v", err)
					time.Sleep(5 * time.Second)
				}
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
			l.logProducer.Shutdown()
			l.awaitShutdown.Done()
			log.Infof("log writer has been stopped")
		case record := <-l.records:
			processingComplete.Add(1)

			log.Debugf("storage provider processing %d bytes", len(record))
			start := time.Now()

			topic := "unknown"
			var dt time.Time
			var eventLog t.EventLogSchema
			err := json.Unmarshal(record, &eventLog)
			if err == nil {
				topic = eventLog.Topic
				index := eventLog.Key.ID
				dt, _ = time.Parse(time.RFC3339, eventLog.Value.Timestamp)
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

				log.Debugf("writing to mix-log-producer-api [index: %v] [docID: %v]: %v", index, docID, string(record))
				_, err = l.logProducer.Write(index, docID, record)
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

	log.Infof("Starting %v writer. Writing logs to %v", l.name, l.cfg.ApiUrl)
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
