package httpclient

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

type Http struct {
	HttpClient http.Client
}

func NewHttp(httpClient http.Client) *Http {
	return &Http{HttpClient: httpClient}
}

// LogRequest implements a formatted string of the HTTP request that's logged for debug purposes
func (h *Http) LogRequest(url *url.URL, method string, body []byte) {
	log.Debugf(">>>>>>> {\"url\": \"%v\", \"method\": \"%v\", \"body\": %v}", url, method, string(body))
}

// LogResponse implements a formatted string of the HTTP response that's logged for debug purposes
func (h *Http) LogResponse(statusCode int, body []byte, err error) {
	log.Debugf("<<<<<<< {\"status_code\": \"%v\", \"error\": \"%v\", \"body\": \"%v\"}", statusCode, err, string(body))
}

// Do handles the sending of the HTTP request (post, get, delete) and processing the response
func (h *Http) Do(request *http.Request) (statusCode int, responseBody []byte, err error) {

	statusCode = 0

	// Send the http request
	timer := prometheus.NewTimer(HttpDurationHistogram.WithLabelValues(request.URL.Path))
	resp, err := h.HttpClient.Do(request)
	monitoring.SetGauge(HttpDurationGauge, timer.ObserveDuration().Seconds(), request.URL.Path)

	if err != nil {
		h.LogResponse(statusCode, responseBody, err)
		return statusCode, responseBody, err
	}
	defer resp.Body.Close()

	// Process the http response
	responseBody, _ = ioutil.ReadAll(resp.Body)
	switch {
	case strings.HasSuffix(request.URL.String(), "/records"):
		h.LogResponse(resp.StatusCode, []byte("records redacted"), err)
	case strings.HasSuffix(request.URL.String(), "/token"):
		h.LogResponse(resp.StatusCode, []byte("token redacted"), err)
	default:
		h.LogResponse(resp.StatusCode, responseBody, err)
	}

	//Prepare and send metric event
	monitoring.IncCounter(HttpRequestsTotalCounter, request.URL.String(), strconv.Itoa(resp.StatusCode), resp.Status)

	return resp.StatusCode, responseBody, err
}

// Post implements an HTTP Post request
func (h *Http) Post(url *url.URL, header http.Header, data []byte) (statusCode int, responseBody []byte, err error) {

	request := http.Request{
		Method: "POST",
		URL:    url,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewBuffer(data)),
	}
	h.LogRequest(request.URL, request.Method, data)

	return h.Do(&request)
}

// Get implements an HTTP Get request
func (h *Http) Get(url *url.URL, header http.Header) (statusCode int, responseBody []byte, err error) {
	request := http.Request{
		Method: "GET",
		URL:    url,
		Header: header,
	}
	h.LogRequest(request.URL, request.Method, nil)

	return h.Do(&request)
}

// Delete implements an HTTP Delete request
func (h *Http) Delete(url *url.URL, header http.Header) (statusCode int, responseBody []byte, err error) {
	request := http.Request{
		Method: "DELETE",
		URL:    url,
		Header: header,
	}
	h.LogRequest(request.URL, request.Method, nil)

	return h.Do(&request)
}
