package log_processors

import (
	//"context"
	"fmt"
	"strings"

	"nuance.xaas-logging.event-log-collector/pkg/log_processors/mix_bi"
	"nuance.xaas-logging.event-log-collector/pkg/log_processors/mix_default"
	"nuance.xaas-logging.event-log-collector/pkg/log_processors/noop"
	pt "nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

func init() {
	Register("mix-default", mix_default.NewLogProcessor)
	Register("mix-bi", mix_bi.NewLogProcessor)
	Register("noop", noop.NewLogProcessor)
}

var processorFactories = make(map[string]pt.LogProcessorFactory)

// Each consumer implementation must Register itself
func Register(name string, factory pt.LogProcessorFactory) {
	log.Infof("Registering consumer factory for %s", name)
	if factory == nil {
		log.Panicf("Consumer factory %s does not exist.", name)
	}
	_, registered := processorFactories[name]
	if registered {
		log.Infof("Processor factory %s already registered. Ignoring.", name)
	}
	processorFactories[name] = factory
}

// CreateLogFetcher is a factory method that will create the named consumer
func CreateLogProcessor(name string, configFile string) (pt.LogProcessor, error) {

	factory, ok := processorFactories[name]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available datastore factories for logging.
		availableProcessors := make([]string, 0)
		for k := range processorFactories {
			availableProcessors = append(availableProcessors, k)
		}
		return nil, fmt.Errorf("invalid processor name. must be one of: %s", strings.Join(availableProcessors, ", "))
	}

	// Run the factory with the configuration.
	return factory(configFile), nil
}
