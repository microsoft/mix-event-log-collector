package provider

import (
	p "nuance.xaas-logging.event-log-collector/pkg/pipeline"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/provider/embedded"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/provider/kafka"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/provider/pulsar"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/provider/redis"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
)

// NewReader() creates an instance of a pipeline reader based on the provider name
// (embedded, kafka, redis, pulsar)
func NewReader(provider string, config p.ReaderParams) io.Reader {
	switch provider {
	case "embedded":
		return embedded.NewReader(config)
	case "kafka":
		return kafka.NewReader(config)
	case "redis":
		return redis.NewReader(config)
	case "pulsar":
		return pulsar.NewReader(config)
	default:
		return nil
	}
}

// NewWriter() creates an instance of a pipeline writer based on the provider name
// (embedded, kafka, redis, pulsar)
func NewWriter(provider string, config p.WriterParams) io.Writer {
	switch provider {
	case "embedded":
		return embedded.NewWriter(config)
	case "kafka":
		return kafka.NewWriter(config)
	case "redis":
		return redis.NewWriter(config)
	case "pulsar":
		return pulsar.NewWriter(config)
	default:
		return nil
	}
}
