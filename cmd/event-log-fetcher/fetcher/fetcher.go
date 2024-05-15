package fetcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"gopkg.in/yaml.v2"
	"nuance.xaas-logging.event-log-collector/pkg/cli"
	fetchers "nuance.xaas-logging.event-log-collector/pkg/log_fetchers"
	"nuance.xaas-logging.event-log-collector/pkg/log_fetchers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	sys "nuance.xaas-logging.event-log-collector/pkg/sys"
)

var (
	// default event log fetcher name is mix-log-api
	DefaultLogFetcherYaml = LogFetcherYaml{
		Fetcher: LogFetcherParams{
			Name: "mix-log-api",
		},
	}
)

// The following structs provide just enough definition
// of the event log fetcher yaml config to quickly
// parse the log fetcher name
type LogFetcherYaml struct {
	Fetcher LogFetcherParams `json:"fetcher" yaml:"fetcher"`
}

type LogFetcherParams struct {
	Name string `json:"name" yaml:"name"`
}

// getNameFromConfigFile() reads in the yaml config file and extracts the
// name of the event log fetcher to create (e.g. mix-log-api)
func getNameFromConfigFile(configFile string) (*string, error) {
	// Start with defaults
	yml := DefaultLogFetcherYaml

	// Read yaml config file content
	file, err := os.ReadFile(filepath.Clean(configFile))
	if err == nil {
		// Unmarshall yaml
		err = yaml.Unmarshal(file, &yml)
		if err != nil {
			return nil, err
		}
	}

	log.Debugf("%+v", yml)

	return &yml.Fetcher.Name, nil
}

// createLogFetcher() creates an instance of a named log fetcher from config
func createLogFetcher() types.LogFetcher {
	log.Debugf("%+v", cli.Args)

	name, err := getNameFromConfigFile(cli.Args.Config)
	if err != nil {
		log.Fatalf("%v", err)
	}

	c, err := fetchers.CreateLogFetcher(*name, cli.Args.Config)
	if err != nil {
		log.Fatalf("%v", err)
	}
	return c
}

/*
 * Run() is the entry point for execution of event-log-fetcher.
 * If using as a library, call fetcher.Run()
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

	// Create and start an instance of event log fetcher
	fetcher := createLogFetcher()
	fetcher.Start()

	// wait for signal to stop fetching records
	signal.ReceiveShutDown() // blocks until shutdown signal
	log.Infof("GRACEFUL SHUTDOWN STARTED [ event-log-fetcher ]....")
	cancelFunc() // Signal cancellation to context.Context
	fetcher.Stop()
	log.Infof("GRACEFUL SHUTDOWN COMPLETE [ event-log-fetcher ]")
}
