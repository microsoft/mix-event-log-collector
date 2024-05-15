package monitoring

/*
 * The monitoring package provides a common set of prometheus labels, collectors, and metric writing methods
 * for a consistent implementation of metrics across all event log collector components.
 */

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"nuance.xaas-logging.event-log-collector/pkg/cli"
	"nuance.xaas-logging.event-log-collector/pkg/logging"
)

const (
	//Prometheus Metric Labels
	PROM_LABEL_ELC_COMPONENT           = "component"
	PROM_LABEL_ELC_COMPONENT_FETCHER   = "fetcher"
	PROM_LABEL_ELC_COMPONENT_PROCESSOR = "processor"
	PROM_LABEL_ELC_COMPONENT_WRITER    = "writer"

	PROM_LABEL_PROCESSOR             = "processor"
	PROM_LABEL_GROUP                 = "group"
	PROM_LABEL_NAME                  = "name"
	PROM_LABEL_TOPIC                 = "topic"
	PROM_LABEL_STATUS                = "status"
	PROM_LABEL_CODE                  = "code"
	PROM_LABEL_MESSAGE               = "message"
	PROM_LABEL_REMOTE_SERVICE        = "remote_service"
	PROM_LABEL_STORAGE               = "storage"
	PROM_LABEL_HTTP_URL              = "http_url"
	PROM_LABEL_HTTP_HEADER_TOKEN     = "http_header_token"
	PROM_LABEL_HTTP_RESPONSE_CODE    = "http_response_code"
	PROM_LABEL_HTTP_RESPONSE_STATUS  = "http_response_status"
	PROM_LABEL_HTTP_RESPONSE_MESSAGE = "http_response_message"

	// Standard error codes
	PROM_ERR_MARSHAL_ERROR        = 520
	PROM_ERR_PIPELINE_WRITE_ERROR = 521
	PROM_ERR_FETCH_ERROR          = 522
	PROM_ERR_RATE_LIMITED         = 523
	PROM_ERR_PROCESS_ERROR        = 524

	// Standard status values
	PROM_STATUS_SUCCESS = "success"
	PROM_STATUS_FAILED  = "failed"

	// Standard message handling status values
	PROM_MSG_PROCESSED     = "processed"
	PROM_MSG_FILTERED      = "filtered"
	PROM_MSG_NOT_PROCESSED = "not_processed"

	// Prometheus metric names
	ELC_LAG_IN      = "elc_lag_in"
	ELC_LAG_IN_HELP = "the message arrival lag for an elc component"

	ELC_LAG_OUT      = "elc_lag_out"
	ELC_LAG_OUT_HELP = "the message departure lag for an elc component"

	ELC_PROCESSING_DURATION      = "elc_message_processing_duration"
	ELC_PROCESSING_DURATION_HELP = "the total time it takes an elc component to process a message"

	ELC_NUM_RECORDS_IN      = "elc_num_records_in"
	ELC_NUM_RECORDS_IN_HELP = "the number of records received by an elc component"

	ELC_NUM_RECORDS_OUT      = "elc_num_records_out"
	ELC_NUM_RECORDS_OUT_HELP = "the number of records sent by an elc component"

	ELC_PROCESSING_TOTAL      = "elc_processing_total"
	ELC_PROCESSING_TOTAL_HELP = "the total number of messages processed"

	ELC_ERRORS      = "elc_processing_errors"
	ELC_ERRORS_HELP = "errors generated in an elc component"
)

var (
	// Instantiations of prometheus metric collectors
	ElcLagIn = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_LAG_IN,
		Help: ELC_LAG_IN_HELP,
	}, []string{
		PROM_LABEL_ELC_COMPONENT,
		PROM_LABEL_TOPIC,
	})

	ElcLagOut = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_LAG_OUT,
		Help: ELC_LAG_OUT_HELP,
	}, []string{
		PROM_LABEL_ELC_COMPONENT,
		PROM_LABEL_TOPIC,
	})

	ElcProcessingTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_PROCESSING_TOTAL,
		Help: ELC_PROCESSING_TOTAL_HELP,
	}, []string{
		PROM_LABEL_ELC_COMPONENT,
		PROM_LABEL_TOPIC,
		PROM_LABEL_STATUS,
		PROM_LABEL_CODE,
		PROM_LABEL_MESSAGE,
	})

	ElcProcessingDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: ELC_PROCESSING_DURATION,
		Help: ELC_PROCESSING_DURATION_HELP,
	}, []string{
		PROM_LABEL_ELC_COMPONENT,
		PROM_LABEL_TOPIC,
		PROM_LABEL_STATUS,
		PROM_LABEL_CODE,
		PROM_LABEL_MESSAGE,
	})

	ElcErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_ERRORS,
		Help: ELC_ERRORS_HELP,
	}, []string{
		PROM_LABEL_ELC_COMPONENT,
		PROM_LABEL_TOPIC,
		PROM_LABEL_CODE,
		PROM_LABEL_MESSAGE,
	})

	ElcNumRecordsInCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_NUM_RECORDS_IN,
		Help: ELC_NUM_RECORDS_IN_HELP,
	}, []string{
		PROM_LABEL_ELC_COMPONENT,
		PROM_LABEL_TOPIC,
	})

	ElcNumRecordsOutCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_NUM_RECORDS_OUT,
		Help: ELC_NUM_RECORDS_OUT_HELP,
	}, []string{
		PROM_LABEL_ELC_COMPONENT,
		PROM_LABEL_TOPIC,
	})
)

var once sync.Once
var reg *prometheus.Registry

// Initialize prometheus registry and collectors
func init() {
	// Create a new registry.
	reg = prometheus.NewRegistry()

	Register(ElcLagIn)
	Register(ElcLagOut)
	Register(ElcNumRecordsInCounter)
	Register(ElcNumRecordsOutCounter)
	Register(ElcProcessingDuration)
	Register(ElcProcessingTotal)
	Register(ElcErrors)
}

// Helper methods that wrap / simplify calling methods of underlying collectors
func Register(collector interface{}) {
	if reg != nil {
		reg.Register(collector.(prometheus.Collector))
	}
}

func SetGauge(gauge *prometheus.GaugeVec, val float64, lvs ...string) {
	gauge.WithLabelValues(lvs...).Set(val)
}

func IncCounter(counter *prometheus.CounterVec, lvs ...string) {
	counter.WithLabelValues(lvs...).Inc()
}

func AddCounter(counter *prometheus.CounterVec, increment float64, lvs ...string) {
	if increment > 0 {
		counter.WithLabelValues(lvs...).Add(increment)
	}
}

// isMonitoringEnabled checks to see if we're running in k8s or if env var ELC_ENABLE_MONITORING exists
func isMonitoringEnabled() bool {
	if _, found := os.LookupEnv("KUBERNETES_PORT"); found {
		return true
	}

	if _, found := os.LookupEnv("ELC_ENABLE_MONITORING"); found {
		return true
	}
	return false
}

// / Start() needs to be called by any event log component that wants monitoring to be running
func Start() {
	// Ensure we only start the monitor one time...
	once.Do(func() {

		// Monitoring is disabled by default when not running in k8s. To enable monitoring when not running in k8s,
		// be sure to set env var ELC_ENABLE_MONITORING
		if !isMonitoringEnabled() {
			logging.Debugf("Monitoring is disabled")
			return
		}
		logging.Debugf("Initializing ..")

		// Add Go module build info.
		reg.MustRegister(collectors.NewBuildInfoCollector())
		reg.MustRegister(collectors.NewGoCollector())

		// Expose the registered metrics via HTTP.
		http.Handle("/metrics", promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{
				// Opt into OpenMetrics to support exemplars.
				EnableOpenMetrics: true,
			},
		))

		// liveness + readiness probes
		http.HandleFunc("/ping", func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusOK)
		})

		go func() {
			srv := &http.Server{
				Addr:         fmt.Sprintf(":%v", cli.Args.Port),
				ReadTimeout:  60 * time.Second,
				WriteTimeout: 60 * time.Second,
			}
			logging.Fatal(srv.ListenAndServe())
		}()
	})
}
