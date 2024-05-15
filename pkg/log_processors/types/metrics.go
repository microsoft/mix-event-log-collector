package types

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

const (
	ELC_LOGS_TOTAL                            = "elc_logs_total"
	ELC_LOGS_TOTAL_HELP                       = "The number of logs being processed in event log collector."
	ELC_LOGS_PROCESSING_DURATION_SECONDS      = "elc_logs_processing_duration_seconds"
	ELC_LOGS_PROCESSING_DURATION_SECONDS_HELP = "the amount of time processing a log in seconds"

	PROCESSED     = "Processed"
	NOT_PROCESSED = "NotProcessed"
)

var (
	LogsTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_LOGS_TOTAL,
		Help: ELC_LOGS_TOTAL_HELP,
	}, []string{
		monitoring.PROM_LABEL_PROCESSOR,
		monitoring.PROM_LABEL_MESSAGE,
		monitoring.PROM_LABEL_STATUS,
	})

	LogsProcessingDurationSecondsGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_LOGS_PROCESSING_DURATION_SECONDS,
		Help: ELC_LOGS_PROCESSING_DURATION_SECONDS_HELP,
	}, []string{
		monitoring.PROM_LABEL_PROCESSOR,
	})
)

func init() {
	monitoring.Register(LogsProcessingDurationSecondsGauge)
	monitoring.Register(LogsTotalCounter)
}
