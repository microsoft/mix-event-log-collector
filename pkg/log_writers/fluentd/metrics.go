package fluentd

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

const (
	ELC_FLUENTD_COMMIT_TOTAL      = "elc_fluentd_commit_total"
	ELC_FLUENTD_COMMIT_TOTAL_HELP = "the number of records committed to fluentd with success or failed status and message"

	ELC_FLUENTD_COMMIT_DURATION_SECONDS      = "elc_fluentd_commit_duration_seconds"
	ELC_FLUENTD_COMMIT_DURATION_SECONDS_HELP = "fluentd commit response time in seconds"
)

var (
	CommitTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_FLUENTD_COMMIT_TOTAL,
		Help: ELC_FLUENTD_COMMIT_TOTAL_HELP,
	}, []string{
		monitoring.PROM_LABEL_CODE,
		monitoring.PROM_LABEL_MESSAGE,
	})

	CommitDurationHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: ELC_FLUENTD_COMMIT_DURATION_SECONDS,
		Help: ELC_FLUENTD_COMMIT_DURATION_SECONDS_HELP,
	})
)

func init() {
	monitoring.Register(CommitTotalCounter)
	monitoring.Register(CommitDurationHistogram)
}
