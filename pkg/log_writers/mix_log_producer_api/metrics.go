package mix_log_producer_api

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

const (
	ELC_MIX_LOG_PRODUCER_API_COMMIT_TOTAL      = "elc_mix_log_producer_api_commit_total"
	ELC_MIX_LOG_PRODUCER_API_COMMIT_TOTAL_HELP = "the number of records committed to mix log producer api with success or failed status and message"

	ELC_MIX_LOG_PRODUCER_API_COMMIT_DURATION_SECONDS      = "elc_mix_log_producer_api_commit_duration_seconds"
	ELC_MIX_LOG_PRODUCER_API_COMMIT_DURATION_SECONDS_HELP = "mix log producer api commit response time in seconds"
)

var (
	CommitTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_MIX_LOG_PRODUCER_API_COMMIT_TOTAL,
		Help: ELC_MIX_LOG_PRODUCER_API_COMMIT_TOTAL_HELP,
	}, []string{
		monitoring.PROM_LABEL_CODE,
		monitoring.PROM_LABEL_MESSAGE,
	})

	CommitDurationHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: ELC_MIX_LOG_PRODUCER_API_COMMIT_DURATION_SECONDS,
		Help: ELC_MIX_LOG_PRODUCER_API_COMMIT_DURATION_SECONDS_HELP,
	})
)

func init() {
	monitoring.Register(CommitTotalCounter)
	monitoring.Register(CommitDurationHistogram)
}
