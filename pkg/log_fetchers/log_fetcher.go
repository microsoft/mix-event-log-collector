package consumers

import (
	"fmt"
	"strings"

	"nuance.xaas-logging.event-log-collector/pkg/log_fetchers/mix_log_api"
	ct "nuance.xaas-logging.event-log-collector/pkg/log_fetchers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

func init() {
	Register("mix-log-api", mix_log_api.NewLogFetcher)
}

var consumerFactories = make(map[string]ct.LogFetcherFactory)

// Each consumer implementation must Register itself
func Register(name string, factory ct.LogFetcherFactory) {
	log.Debugf("Registering consumer factory for %s", name)
	if factory == nil {
		log.Panicf("Consumer factory %s does not exist.", name)
	}
	_, registered := consumerFactories[name]
	if registered {
		log.Debugf("Consumer factory %s already registered. Ignoring.", name)
	}
	consumerFactories[name] = factory
}

// CreateLogFetcher is a factory method that will create the named consumer
func CreateLogFetcher(name string, configFile string) (ct.LogFetcher, error) {

	factory, ok := consumerFactories[name]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available datastore factories for logging.
		availableConsumers := make([]string, 0)
		for k := range consumerFactories {
			availableConsumers = append(availableConsumers, k)
		}
		return nil, fmt.Errorf("invalid consumer name. must be one of: %s", strings.Join(availableConsumers, ", "))
	}

	// Run the factory with the configuration.
	return factory(configFile), nil
}
