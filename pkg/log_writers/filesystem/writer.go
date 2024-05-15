package filesystem

/*
 * filesystem writer implements a log writer that uses the local filesystem
 * as the event log storage provider. Use a filesystem log writer for
 * local event log collection (e.g. dev/debug activities)
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
	"nuance.xaas-logging.event-log-collector/pkg/oauth"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io"
	iot "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/types"
)

var (
	// Default filesystem writer config
	DefaultLogWriterYaml = LogWriterYaml{
		Writer: LogWriterParams{
			Name: "filesystem",
			Config: Config{
				NumWorkers: 5,
				Path:       "event-logs",
				AudioFetcherParams: &AudioFetcherParams{
					Enabled: true,
					Credentials: Credentials{
						AuthDisabled:  false,
						TokenURL:      "https://auth.crt.nuance.com/oauth2/token",
						Scope:         "log",
						AuthTimeoutMS: 5000,
					},
					ApiUrl: "https://log.api.nuance.com/api/v1/audio",
				},
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

type AudioFetcherParams struct {
	Enabled     bool        `json:"enabled" yaml:"enabled"`
	Credentials Credentials `json:"credentials" yaml:"credentials"`
	ApiUrl      string      `json:"api_url" yaml:"api_url"`
}

type Config struct {
	NumWorkers         int                 `json:"num_workers" yaml:"num_workers"`
	Path               string              `json:"path" yaml:"path"`
	AudioFetcherParams *AudioFetcherParams `json:"audio_fetcher" yaml:"audio_fetcher"`
}

// LogWriter is the filesystem LogWriter implementation
type LogWriter struct {
	name string
	cfg  Config

	records chan []byte

	// signaling
	shutdown         chan bool
	jobHandlerSignal chan bool
	awaitShutdown    sync.WaitGroup

	reader     iot.Reader
	filesystem wt.Storage

	audioFetcher *AudioFetcher
}

//// Public methods

// NewLogWriter() creates a new instance of a filesystem LogWriter
func NewLogWriter(configFile string) wt.LogWriter {
	log.Debugf("factory creating new filesystem log writer")

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
		filesystem:       NewStorage(cfg.Writer.Config.Path),
	}
	writer.reader, err = io.CreateReader(pipeline.WRITER, configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}

	if cfg.Writer.Config.AudioFetcherParams != nil && cfg.Writer.Config.AudioFetcherParams.Enabled {
		auth := oauth.NewAuthenticator(oauth.Credentials(cfg.Writer.Config.AudioFetcherParams.Credentials))
		writer.audioFetcher = NewAudioFetcher(auth, cfg.Writer.Config.AudioFetcherParams.ApiUrl)
	}

	return &writer
}

//// Helper methods

// getValueFromEnv() returns the value of the env var if available. If not available, returns the default value
func getValueFromEnv(name string, defaultVal string) string {
	if val, found := os.LookupEnv(name); found {
		return val
	}
	return defaultVal
}

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
	yml.Writer.Config.AudioFetcherParams.Credentials.ClientID = getValueFromEnv("ELC_CLIENT_ID", yml.Writer.Config.AudioFetcherParams.Credentials.ClientID)
	yml.Writer.Config.AudioFetcherParams.Credentials.ClientSecret = getValueFromEnv("ELC_CLIENT_SECRET", yml.Writer.Config.AudioFetcherParams.Credentials.ClientSecret)
	yml.Writer.Config.AudioFetcherParams.Credentials.TokenURL = getValueFromEnv("ELC_TOKEN_URL", yml.Writer.Config.AudioFetcherParams.Credentials.TokenURL)
	yml.Writer.Config.AudioFetcherParams.ApiUrl = getValueFromEnv("ELC_AFSS_URL", yml.Writer.Config.AudioFetcherParams.ApiUrl)

	raw, _ := json.Marshal(log.MaskSensitiveData(yml))
	log.Debugf("%v", string(raw))

	return &yml, nil
}

// parseSessionIDAndServicePath parses the dialog session id and asr session id, if available for the corresponding
// event log. It also parses the service type. It then takes the information available to construct a
// path containing the service type (e.g. asr, nlu, tts, dlg, nii) and any available session id's
func (l *LogWriter) parseSessionIDAndServicePath(eventLog types.EventLogSchema) *string {
	var dlgSessionID, asrSessionID *string
	var service string

	// Grab dialog session id from client data if it exists
	switch {
	case eventLog.Value.Data.ClientData != nil:
		if sid, ok := eventLog.Value.Data.ClientData["x-nuance-dialog-session-id"].(string); ok {
			dlgSessionID = &sid
		}
	case eventLog.Value.Data.Request != nil && eventLog.Value.Data.Request.ClientData != nil:
		if sid, ok := eventLog.Value.Data.Request.ClientData["x-nuance-dialog-session-id"].(string); ok {
			dlgSessionID = &sid
		}
	}

	// Parse service type, and dialog and asr session id's if available
	switch eventLog.Value.Service {
	case "DLGaaS":
		if eventLog.Value.Source == "NIIEventLogger" {
			service = "nii"
		} else {
			service = "dlg"
		}
		dlgSessionID = &eventLog.Value.Data.SessionID
	case "ASRaaS":
		service = "asr"
		asrSessionID = &eventLog.Value.Data.AsrSessionID
	case "TTSaaS":
		service = "tts"
	case "NLUaaS":
		service = "nlu"
	}

	// Build the path based on information available
	var path string
	switch {
	case dlgSessionID == nil && asrSessionID == nil:
		path = service
	case dlgSessionID == nil && asrSessionID != nil:
		path = fmt.Sprintf("%v/%v", service, *asrSessionID)
	case dlgSessionID != nil && asrSessionID == nil:
		path = fmt.Sprintf("%v/%v", *dlgSessionID, service)
	case dlgSessionID != nil && asrSessionID != nil:
		path = fmt.Sprintf("%v/%v/%v", *dlgSessionID, service, *asrSessionID)
	}
	return &path
}

// getIndex() creates a record index based on timestamp. If a dialog session id exists, it will be
// included as part of the index name. For filesystem, this is the path that the record will be stored under
func (l *LogWriter) getIndex(eventLog types.EventLogSchema) string {
	dt, _ := time.Parse(time.RFC3339, eventLog.Value.Timestamp)
	sessionID := l.parseSessionIDAndServicePath(eventLog)

	if sessionID != nil {
		return fmt.Sprintf("%v/%v/%v", eventLog.Topic, dt.Format("01-02-2006"), *sessionID)
	}
	return dt.Format("01-02-2006")
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

// fetchAudio() downloads audio from AFSS
func (l *LogWriter) fetchAudio(eventLog types.EventLogSchema, root string) {

	// Range thru list of recognize objects and process each audio urn
	for idx, obj := range eventLog.Value.Data.CallSummary.Recognize {
		// extract audio urn
		// e.g. "audioUrn":"urn:nuance-mix:log:service:audio/asraas/0255f5af-aaa9-43d7-b5f8-a0d939790550"

		// audio urn may not exist if call recording has been suppresssed
		if obj.(types.JsonObj)["audioUrn"] == nil {
			return
		}

		audioUrn := obj.(types.JsonObj)["audioUrn"].(string)

		// extract audio rate
		recoParams := obj.(types.JsonObj)["recognitionParameters"].(types.JsonObj)
		audioFormat := recoParams["audioFormat"].(string)
		rate := 8000
		switch {
		case strings.Contains(audioFormat, "16000"):
			rate = 16000
		}

		// build full path name of audio file
		dst := filepath.Join(root, fmt.Sprintf("audio_%v.wav", idx))

		// download the audio to file
		err := l.audioFetcher.Download(audioUrn, dst, rate)
		if err != nil {
			log.Errorf("%v", err)
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
			l.filesystem.Shutdown()
			log.Infof("log writer has been stopped")
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

				_, err = l.filesystem.Write(index, fmt.Sprintf("%v.json", docID), record)

				// If we have an ASR callsummary record and audio fetching is enabled, download the audio
				switch eventLog.Value.Service {
				case "ASRaaS":
					if l.audioFetcher != nil && strings.Contains(eventLog.Value.Data.DataContentType, "callsummary") {
						l.fetchAudio(eventLog, filepath.Join(l.cfg.Path, index))
					}
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

	log.Infof("Starting %v writer. Writing logs to %v", l.name, l.cfg.Path)
	return nil
}

// Stop() implements the LogWriter interface. It shuts down event log writing
func (l *LogWriter) Stop() {
	l.shutdown <- true
	l.awaitShutdown.Wait()
}

// GetStatus() implements the LogWriter interface. Currently not doing much...
func (l *LogWriter) GetStatus() (wt.LogWriterStatus, error) {
	return wt.LogWriterStatus{}, nil
}
