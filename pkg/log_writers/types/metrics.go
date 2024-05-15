package types

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

const (
	ELC_LOG_WRITES_TOTAL      = "elc_log_writes_total"
	ELC_LOG_WRITES_TOTAL_HELP = "The number of logs written to storage by event log collector."

	ELC_STORAGE_REMOTE_CONNECTION      = "elc_storage_remote_connection"
	ELC_STORAGE_REMOTE_CONNECTION_HELP = "The current successful connections from elc storage clients to remote service like elastic search, mix3 log api"

	ELC_STORAGE_WRITE_DURATION_SECONDS      = "elc_storage_write_duration_seconds"
	ELC_STORAGE_WRITE_DURATION_SECONDS_HELP = "current write duration in seconds for storages such as filesystem, s3/minio elastic search, mix3 log api"

	WRITTEN     = "Written"
	NOT_WRITTEN = "NotWritten"

	ERR_NETWORK = "NetworkError"

	LABEL_ELASTICSEARCH        = "elasticsearch"
	LABEL_FILESYSTEM           = "filesystem"
	LABEL_FLUENTD              = "fluentd"
	LABEL_MIX_LOG_PRODUCER_API = "mix_log_producer_api"
	LABEL_MONGODB              = "mongodb"
	LABEL_NEAP                 = "neap"
)

var RemoteConnectionGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: ELC_STORAGE_REMOTE_CONNECTION,
	Help: ELC_STORAGE_REMOTE_CONNECTION_HELP,
}, []string{
	monitoring.PROM_LABEL_REMOTE_SERVICE,
})

var WriteDurationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: ELC_STORAGE_WRITE_DURATION_SECONDS,
	Help: ELC_STORAGE_WRITE_DURATION_SECONDS_HELP,
}, []string{
	monitoring.PROM_LABEL_STORAGE,
})

var WritesTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: ELC_LOG_WRITES_TOTAL,
	Help: ELC_LOG_WRITES_TOTAL_HELP,
}, []string{
	monitoring.PROM_LABEL_PROCESSOR,
	monitoring.PROM_LABEL_MESSAGE,
	monitoring.PROM_LABEL_STATUS,
})

func init() {
	monitoring.Register(WritesTotalCounter)
	monitoring.Register(RemoteConnectionGauge)
	monitoring.Register(WriteDurationGauge)
}
