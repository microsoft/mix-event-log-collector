package elasticsearch

/*
 * elasticsearch writer implements a log writer that writes event logs to
 * elasticsearch as the storage provider.
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
	"nuance.xaas-logging.event-log-collector/pkg/ratelimiter"
	t "nuance.xaas-logging.event-log-collector/pkg/types"
)

var (
	// Default elasticsearch writer config
	DefaultLogWriterYaml = LogWriterYaml{
		Writer: LogWriterParams{
			Name: "elasticsearch",
			Config: Config{
				NumWorkers: 5,
				RateLimit: ratelimiter.RateLimit{
					Limit: 10,
					Burst: 1,
				},
				DocType:           "mix3-record",
				IndexPrefix:       "mix3-logs-v2",
				AppendDateToIndex: true,
				Refresh:           true,
				Addresses:         append(make([]string, 1), "http://localhost:9200"),
				MaxRetries:        10,
				Delay:             10,
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

// TLSConfig is used to configure TLS connections.
type TLSConfig struct {

	// ServerName is used to verify the hostname on the returned
	// certificates unless InsecureSkipVerify is given. It is also included
	// in the client's handshake to support virtual hosting unless it is
	// an IP address.
	ServerName string `json:"ServerName" yaml:"server_name"`

	// InsecureSkipVerify controls whether a client verifies the server's
	// certificate chain and host name. If InsecureSkipVerify is true, crypto/tls
	// accepts any certificate presented by the server and any host name in that
	// certificate. In this mode, TLS is susceptible to machine-in-the-middle
	// attacks unless custom verification is used. This should be used only for
	// testing or in combination with VerifyConnection or VerifyPeerCertificate.
	InsecureSkipVerify bool `json:"InsecureSkipVerify" yaml:"insecure_skip_verify"`
}

// Config is the configuration of the elasticsearch writer
type Config struct {
	NumWorkers        int                   `json:"num_workers" yaml:"num_workers"`
	RateLimit         ratelimiter.RateLimit `json:"rate_limit" yaml:"rate_limit"`
	DocType           string                `json:"doc_type" yaml:"doc_type"`
	IndexPrefix       string                `json:"index_prefix" yaml:"index_prefix"`
	AppendDateToIndex bool                  `json:"append_date_to_index" yaml:"append_date_to_index"`
	Refresh           bool                  `json:"refresh,omitempty"`
	Addresses         []string              `json:"addresses" yaml:"addresses"`
	MaxRetries        int                   `json:"max_retries" yaml:"max_retries"`
	Delay             int                   `json:"delay" yaml:"delay"`
	Pipeline          *string               `json:"pipeline" yaml:"pipeline"`

	// overlay of the elasticsearch.Config struct
	Username                *string    `json:"username" yaml:"username"`
	Password                *string    `json:"password" yaml:"password"`
	CloudID                 *string    `json:"CloudID" yaml:"cloud_id"`                               // Endpoint for the Elastic Service (https://elastic.co/cloud).
	APIKey                  *string    `json:"APIKey" yaml:"api_key"`                                 // Base64-encoded token for authorization; if set, overrides username/password and service token.
	ServiceToken            *string    `json:"ServiceToken" yaml:"service_token"`                     // Service token for authorization; if set, overrides username/password.
	CertificateFingerprint  *string    `json:"CertificateFingerprint" yaml:"certificate_fingerprint"` // SHA256 hex fingerprint given by Elasticsearch on first launch.
	RetryOnStatus           []int      `json:"RetryOnStatus" yaml:"retry_on_status"`                  // List of status codes for retry. Default: 502, 503, 504.
	DisableRetry            bool       `json:"DisableRetry" yaml:"disable_retry"`                     // Default: false.
	EnableRetryOnTimeout    bool       `json:"EnableRetryOnTimeout" yaml:"enable_retry_on_timeout"`   // Default: false.
	CompressRequestBody     bool       `json:"CompressRequestBody" yaml:"compress_request_body"`      // Default: false.
	DiscoverNodesOnStart    bool       `json:"DiscoverNodesOnStart" yaml:"discover_nodes_on_start"`   // Discover nodes when initializing the client. Default: false.
	EnableMetrics           bool       `json:"EnableMetrics" yaml:"enable_metrics"`                   // Enable the metrics collection.
	EnableDebugLogger       bool       `json:"EnableDebugLogger" yaml:"enable_debug_logger"`          // Enable the debug logging.
	UseResponseCheckOnly    bool       `json:"UseResponseCheckOnly" yaml:"use_response_check_only"`
	EnableCompatibilityMode bool       `json:"EnableCompatibilityMode" yaml:"enable_compatibility_mode"` // Enable sends compatibility header
	DisableMetaHeader       bool       `json:"DisableMetaHeader" yaml:"disable_meta_header"`             // Disable the additional "X-Elastic-Client-Meta" HTTP header.
	TLSConfig               *TLSConfig `json:"tls_config,omitempty" yaml:"tls_config"`
}

// LogWriter is the elasticsearch LogWriter implementation
type LogWriter struct {
	name string
	cfg  Config

	records chan []byte

	// signaling
	shutdown         chan bool
	jobHandlerSignal chan bool
	awaitShutdown    sync.WaitGroup

	reader        iot.Reader
	elasticsearch wt.Storage
}

//// Public methods

// NewLogWriter() creates a new instance of an elasticsearch LogWriter
func NewLogWriter(configFile string) wt.LogWriter {
	log.Infof("factory creating new elasticsearch log writer")

	cfg, err := loadConfigFromFile(configFile)
	if err != nil {
		log.Panicf("%v", err)
	}

	writer := LogWriter{
		name:             cfg.Writer.Name,
		cfg:              cfg.Writer.Config,
		shutdown:         make(chan bool),
		jobHandlerSignal: make(chan bool),
		records:          make(chan []byte, cfg.Writer.Config.NumWorkers),
		elasticsearch:    NewStorage(cfg.Writer.Config),
	}
	writer.reader, err = io.CreateReader(pipeline.WRITER, configFile)
	if err != nil {
		log.Panicf("%v", err)
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

	// Over-ride some writer config values with env vars if they've been set
	yml.Writer.Config.Username = getValueFromEnv("ELC_ELASTICSEARCH_USERNAME", yml.Writer.Config.Username)
	yml.Writer.Config.Password = getValueFromEnv("ELC_ELASTICSEARCH_PASSWORD", yml.Writer.Config.Password)
	yml.Writer.Config.APIKey = getValueFromEnv("ELC_ELASTICSEARCH_API_KEY", yml.Writer.Config.APIKey)
	yml.Writer.Config.CloudID = getValueFromEnv("ELC_ELASTICSEARCH_CLOUD_ID", yml.Writer.Config.CloudID)
	yml.Writer.Config.CertificateFingerprint = getValueFromEnv("ELC_ELASTICSEARCH_CERT_FINGERPRINT", yml.Writer.Config.CertificateFingerprint)
	yml.Writer.Config.ServiceToken = getValueFromEnv("ELC_ELASTICSEARCH_SERVICE_TOKEN", yml.Writer.Config.ServiceToken)

	raw, _ := json.Marshal(log.MaskSensitiveData(yml))
	log.Debugf("%v", string(raw))

	return &yml, nil
}

// getValueFromEnv() returns the value of the env var if available. If not available, returns the default value
func getValueFromEnv(name string, defaultVal *string) *string {
	if val, found := os.LookupEnv(name); found {
		return &val
	}
	return defaultVal
}

// getIndex() creates a record index based on index prefix and timestamp.
func (l *LogWriter) getIndex(timestamp time.Time) string {
	if l.cfg.AppendDateToIndex {
		return fmt.Sprintf("%s-%s", l.cfg.IndexPrefix, timestamp.Format("01-02-2006"))
	}
	return l.cfg.IndexPrefix
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
			l.elasticsearch.Shutdown()
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
				dt, _ = time.Parse(time.RFC3339, eventLog.Value.Timestamp)
				index := l.getIndex(dt)
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

				log.Debugf("writing to elasticsearch [index: %v] [docID: %v]: %v", index, docID, string(record))
				_, err = l.elasticsearch.Write(index, docID, record)
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

	log.Infof("Starting %v writer. Writing logs of doc_type %v to index %v", l.name, l.cfg.DocType, l.cfg.IndexPrefix)
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
