package processor

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"gopkg.in/yaml.v2"
	"nuance.xaas-logging.event-log-collector/pkg/cli"
	processors "nuance.xaas-logging.event-log-collector/pkg/log_processors"
	"nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	sys "nuance.xaas-logging.event-log-collector/pkg/sys"
)

var (
	// default event log processor name is mix-default
	DefaultProcessorYaml = LogProcessorYaml{
		Processor: LogProcessorParams{
			Name: "mix-default",
		},
	}
)

// The following structs provide just enough definition
// of the event log processor yaml config to quickly
// parse the log processor name
type LogProcessorYaml struct {
	Processor LogProcessorParams `json:"processor" yaml:"processor"`
}

type LogProcessorParams struct {
	Name string `json:"name" yaml:"name"`
}

// getNameFromConfigFile() reads in the yaml config file and extracts the
// name of the event log processor to create (e.g. mix-default)
func getNameFromConfigFile(configFile string) (*string, error) {
	// Start with defaults
	yml := DefaultProcessorYaml

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

	return &yml.Processor.Name, nil
}

// createLogProcessor() creates an instance of a named log processor from config
func createLogProcessor() types.LogProcessor {
	log.Debugf("%+v", cli.Args)

	name, err := getNameFromConfigFile(cli.Args.Config)
	if err != nil {
		log.Fatalf("%v", err)
	}

	p, err := processors.CreateLogProcessor(*name, cli.Args.Config)
	if err != nil {
		log.Fatalf("%v", err)
	}
	return p
}

/*
 * Run() is the entry point for execution of event-log-processor.
 * If using as a library, call processor.Run()
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

	// Create and start an instance of event log processor
	processor := createLogProcessor()
	processor.Start()

	// wait for signal to stop processing
	signal.ReceiveShutDown() // blocks until shutdown signal
	log.Infof("GRACEFUL SHUTDOWN STARTED [ event-log-processor ]....")
	cancelFunc() // Signal cancellation to context.Context
	processor.Stop()
	log.Infof("GRACEFUL SHUTDOWN COMPLETE [ event-log-processor ]")
}
