package opensearch

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

const (
	ELC_OS_COMMIT_TOTAL      = "elc_opensearch_commit_total"
	ELC_OS_COMMIT_TOTAL_HELP = "the number of records committed to opensearch with success or failed status and message"

	ELC_OS_COMMIT_DURATION_SECONDS      = "elc_opensearch_commit_duration_seconds"
	ELC_OS_COMMIT_DURATION_SECONDS_HELP = "opensearch commit response time in seconds"
)

var (
	CommitTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_OS_COMMIT_TOTAL,
		Help: ELC_OS_COMMIT_TOTAL_HELP,
	}, []string{
		monitoring.PROM_LABEL_CODE,
		monitoring.PROM_LABEL_MESSAGE,
	})

	CommitDurationHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: ELC_OS_COMMIT_DURATION_SECONDS,
		Help: ELC_OS_COMMIT_DURATION_SECONDS_HELP,
	})
)

func init() {
	monitoring.Register(CommitTotalCounter)
	monitoring.Register(CommitDurationHistogram)
}
