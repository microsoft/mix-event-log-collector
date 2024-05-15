package httpclient

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	//m "nuance.xaas-logging.event-log-collector/pkg/pipeline/monitoring"
)

const (
	ELC_HTTP_REQUESTS_TOTAL      = "elc_http_requests_total"
	ELC_HTTP_REQUESTS_TOTAL_HELP = "the number of request made with response code and message."

	ELC_HTTP_REQUEST_DURATION_SECONDS      = "elc_http_request_duration_seconds"
	ELC_HTTP_REQUEST_DURATION_SECONDS_HELP = "http request duration in seconds"

	ELC_HTTP_REQUEST_DURATION_SECONDS_CURRENT      = "elc_http_request_duration_seconds_current"
	ELC_HTTP_REQUEST_DURATION_SECONDS_CURRENT_HELP = "the lastly observed http request duration in seconds"

	PROM_LABEL_HTTP_URL              = "http_url"
	PROM_LABEL_HTTP_HEADER_TOKEN     = "http_header_token"
	PROM_LABEL_HTTP_RESPONSE_CODE    = "http_response_code"
	PROM_LABEL_HTTP_RESPONSE_STATUS  = "http_response_status"
	PROM_LABEL_HTTP_RESPONSE_MESSAGE = "http_response_message"
)

var (
	HttpRequestsTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ELC_HTTP_REQUESTS_TOTAL,
		Help: ELC_HTTP_REQUESTS_TOTAL_HELP,
	}, []string{
		PROM_LABEL_HTTP_URL,
		PROM_LABEL_HTTP_RESPONSE_CODE,
		PROM_LABEL_HTTP_RESPONSE_STATUS,
	})

	HttpDurationHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: ELC_HTTP_REQUEST_DURATION_SECONDS,
			Help: ELC_HTTP_REQUEST_DURATION_SECONDS_HELP,
		}, []string{
			"path",
		})
	HttpDurationGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: ELC_HTTP_REQUEST_DURATION_SECONDS_CURRENT,
			Help: ELC_HTTP_REQUEST_DURATION_SECONDS_CURRENT_HELP,
		}, []string{
			"path",
		})
)

func init() {
	monitoring.Register(HttpRequestsTotalCounter)
	monitoring.Register(HttpDurationHistogram)
	monitoring.Register(HttpDurationGauge)
}
