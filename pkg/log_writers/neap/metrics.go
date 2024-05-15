package neap

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

const (
	ELC_NEAP_COMMIT_TOTAL      = "elc_neap_commit_total"
	ELC_NEAP_COMMIT_TOTAL_HELP = "the number of records committed to neap with success or failed status and message"

	ELC_NEAP_COMMIT_DURATION_SECONDS      = "elc_neap_commit_duration_seconds"
	ELC_NEAP_COMMIT_DURATION_SECONDS_HELP = "neap commit response time in seconds"
)

var (
	CommitTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_NEAP_COMMIT_TOTAL,
		Help: ELC_NEAP_COMMIT_TOTAL_HELP,
	}, []string{
		monitoring.PROM_LABEL_CODE,
		monitoring.PROM_LABEL_MESSAGE,
	})

	CommitDurationHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: ELC_NEAP_COMMIT_DURATION_SECONDS,
		Help: ELC_NEAP_COMMIT_DURATION_SECONDS_HELP,
	})
)

func init() {
	monitoring.Register(CommitTotalCounter)
	monitoring.Register(CommitDurationHistogram)
}
