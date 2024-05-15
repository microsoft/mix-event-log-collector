package writer

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"gopkg.in/yaml.v2"
	"nuance.xaas-logging.event-log-collector/pkg/cli"
	writers "nuance.xaas-logging.event-log-collector/pkg/log_writers"
	"nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	sys "nuance.xaas-logging.event-log-collector/pkg/sys"
)

var (
	// default event log writer name is filesystem
	DefaultLogWriterYaml = LogWriterYaml{
		Writer: LogWriterParams{
			Name: "filesystem",
		},
	}
)

// The following structs provide just enough definition
// of the event log writer yaml config to quickly
// parse the log writer name
type LogWriterYaml struct {
	Writer LogWriterParams `json:"writer" yaml:"writer"`
}

type LogWriterParams struct {
	Name string `json:"name" yaml:"name"`
}

// getNameFromConfigFile() reads in the yaml config file and extracts the
// name of the event log writer to create (e.g. elasticsearch)
func getNameFromConfigFile(configFile string) (*string, error) {
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

	log.Debugf("%+v", yml)

	return &yml.Writer.Name, nil
}

// createLogWriter() creates an instance of a named log writer from config
func createLogWriter() types.LogWriter {
	log.Debugf("%+v", cli.Args)

	name, err := getNameFromConfigFile(cli.Args.Config)
	if err != nil {
		log.Fatalf("%v", err)
	}

	w, err := writers.CreateLogWriter(*name, cli.Args.Config)
	if err != nil {
		log.Fatalf("%v", err)
	}
	return w
}

/*
 * Run() is the entry point for execution of event-log-writer.
 * If using as a library, call writer.Run()
 */
func Run(wg *sync.WaitGroup) {
	defer wg.Done()

	// Initialize logging and monitoring
	// Monitoring is enabled by default when deploye din k8s, and disabled by default when run locally
	// To force enabling of monitoring, create env var ELC_ENABLE_MONITORING
	log.Setup()
	monitoring.Start()

	// Setup signaling to capture program kill events and allow for graceful shutdown
	signal := sys.NewSignal(syscall.SIGINT, syscall.SIGTERM, syscall.SIGILL)

	// context with cancel for graceful shutdown
	_, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// Create and start an instance of event log writer
	writer := createLogWriter()
	writer.Start() //Start(events, err chan)

	// wait for signal to stop consuming/writing
	signal.ReceiveShutDown() // blocks until shutdown signal
	log.Infof("GRACEFUL SHUTDOWN STARTED [ event-log-writer ]....")
	cancelFunc() // Signal cancellation to context.Context
	writer.Stop()
	log.Infof("GRACEFUL SHUTDOWN COMPLETE [ event-log-writer ]")
}
