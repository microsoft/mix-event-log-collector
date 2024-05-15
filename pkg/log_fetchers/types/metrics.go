package types

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

const (
	COMMITTED   = "Committed"
	UNCOMMITTED = "UnCommitted"

	ELC_FETCHER_OFFSET_LOW      = "elc_fetcher_offset_low"
	ELC_FETCHER_OFFSET_LOW_HELP = "the lastly observed low offset of topic and partition"

	ELC_FETCHER_OFFSET_CURRENT      = "elc_fetcher_offset_current"
	ELC_FETCHER_OFFSET_CURRENT_HELP = "the lastly observed current offset of topic and partition"

	ELC_FETCHER_OFFSET_HIGH      = "elc_fetcher_offset_high"
	ELC_FETCHER_OFFSET_HIGH_HELP = "the lastly observed high offset of topic and partition"

	ELC_FETCHER_OFFSET_BEHIND      = "elc_fetcher_offset_behind"
	ELC_FETCHER_OFFSET_BEHIND_HELP = "the number of backlogs to consume of topic and partition"

	ELC_COMMIT_TOTAL      = "elc_commit_total"
	ELC_COMMIT_TOTAL_HELP = "the number of kafka manual commit made to remote service like kafka, mix3 log api"

	ELC_PARTITIONS_TOTAL      = "elc_partitions_total"
	ELC_PARTITIONS_TOTAL_HELP = "The number of partitions of a topic in kafka"
)

var (
	ConsumerOffsetLowGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_FETCHER_OFFSET_LOW,
		Help: ELC_FETCHER_OFFSET_LOW_HELP,
	}, []string{
		"topic",
		"partition",
	})
	ConsumerOffsetCurrentGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_FETCHER_OFFSET_CURRENT,
		Help: ELC_FETCHER_OFFSET_CURRENT_HELP,
	}, []string{
		"topic",
		"partition",
	})
	ConsumerOffsetHighGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_FETCHER_OFFSET_HIGH,
		Help: ELC_FETCHER_OFFSET_HIGH_HELP,
	}, []string{
		"topic",
		"partition",
	})
	ConsumerOffsetBehindGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_FETCHER_OFFSET_BEHIND,
		Help: ELC_FETCHER_OFFSET_BEHIND_HELP,
	}, []string{
		"topic",
		"partition",
	})

	KafkaCommitTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_COMMIT_TOTAL,
		Help: ELC_COMMIT_TOTAL_HELP,
	}, []string{
		monitoring.PROM_LABEL_REMOTE_SERVICE,
		monitoring.PROM_LABEL_STATUS,
		monitoring.PROM_LABEL_MESSAGE,
	})

	PartitionTotalGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_PARTITIONS_TOTAL,
		Help: ELC_PARTITIONS_TOTAL_HELP,
	}, []string{
		monitoring.PROM_LABEL_REMOTE_SERVICE,
		monitoring.PROM_LABEL_TOPIC,
		monitoring.PROM_LABEL_GROUP,
		monitoring.PROM_LABEL_NAME,
	})
)

//Create and register metrics
func init() {
	monitoring.Register(ConsumerOffsetLowGauge)
	monitoring.Register(ConsumerOffsetCurrentGauge)
	monitoring.Register(ConsumerOffsetHighGauge)
	monitoring.Register(ConsumerOffsetBehindGauge)
	monitoring.Register(PartitionTotalGauge)
}
