package log_writers

import (
	"fmt"
	"strings"

	"nuance.xaas-logging.event-log-collector/pkg/log_writers/elasticsearch"
	"nuance.xaas-logging.event-log-collector/pkg/log_writers/filesystem"
	"nuance.xaas-logging.event-log-collector/pkg/log_writers/fluentd"
	"nuance.xaas-logging.event-log-collector/pkg/log_writers/mix_log_producer_api"
	"nuance.xaas-logging.event-log-collector/pkg/log_writers/mongodb"
	"nuance.xaas-logging.event-log-collector/pkg/log_writers/neap"
	"nuance.xaas-logging.event-log-collector/pkg/log_writers/opensearch"
	t "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

func init() {
	Register("filesystem", filesystem.NewLogWriter)
	Register("elasticsearch", elasticsearch.NewLogWriter)
	Register("opensearch", opensearch.NewLogWriter)
	Register("mongodb", mongodb.NewLogWriter)
	Register("fluentd", fluentd.NewLogWriter)
	Register("neap", neap.NewLogWriter)
	Register("mix-log-producer-api", mix_log_producer_api.NewLogWriter)
}

var writerFactories = make(map[string]t.LogWriterFactory)

// Each consumer implementation must Register itself
func Register(name string, factory t.LogWriterFactory) {
	log.Infof("Registering log writer factory for %s", name)
	if factory == nil {
		log.Panicf("Log writer factory %s does not exist.", name)
	}
	_, registered := writerFactories[name]
	if registered {
		log.Infof("Processor factory %s already registered. Ignoring.", name)
	}
	writerFactories[name] = factory
}

// CreateLogWriter is a factory method that will create the named writer
func CreateLogWriter(name string, configFile string) (t.LogWriter, error) {

	factory, ok := writerFactories[name]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available datastore factories for logging.
		availableWriters := make([]string, 0)
		for k := range writerFactories {
			availableWriters = append(availableWriters, k)
		}
		return nil, fmt.Errorf("invalid log writer name. must be one of: %s", strings.Join(availableWriters, ", "))
	}

	// Run the factory with the configuration.
	return factory(configFile), nil
}
