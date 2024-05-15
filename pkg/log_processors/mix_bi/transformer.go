package mix_bi

/*
 * Mix-bi transformer reduces and transforms event log records into smaller records containing key
 * metrics for delivering business analytics
 */

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tp "nuance.xaas-logging.event-log-collector/pkg/types"
)

type Value struct {
	ID        string `json:"id,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// Metric is the target data object represensation of the original event log record.
// The structure of a metric must mirror an EventLogSchema (Value, Paritition, Offset, ServiceType,
// and Topic) for it to pass validation in downstream components (e.g. log processor, log writer)
type Metric struct {
	Value                Value      `json:"value,omitempty"`
	Partition            float64    `json:"partition,omitempty"`
	Offset               float64    `json:"offset,omitempty"`
	ServiceType          string     `json:"service_type,omitempty"`
	Topic                string     `json:"topic,omitempty"`
	ProcessingDurationMs int        `json:"processing_duration_ms,omitempty"`
	StatusCode           int        `json:"status_code,omitempty"`
	StatusMessage        string     `json:"status_message,omitempty"`
	StatusDetails        string     `json:"status_details,omitempty"`
	LanguageCode         string     `json:"language_code,omitempty"`
	ClientData           tp.JsonObj `json:"client_data,omitempty"`
}

// NewMetric() creates an instance of a new metric record
func NewMetric(eventLog tp.EventLogSchema) Metric {
	metric := Metric{
		Value: Value{
			ID:        eventLog.Key.ID,
			Timestamp: eventLog.Value.Data.ProcessingTime.StartTime,
		},
		Topic:                eventLog.Topic,
		Partition:            eventLog.Partition,
		Offset:               eventLog.Offset,
		ServiceType:          eventLog.Value.Service,
		ProcessingDurationMs: int(eventLog.Value.Data.ProcessingTime.DurationMs),
		StatusCode:           eventLog.Value.Data.Response.Status.Code,
		StatusMessage:        eventLog.Value.Data.Response.Status.Message,
		StatusDetails:        eventLog.Value.Data.Response.Status.Details,
		LanguageCode:         "",
		ClientData:           nil,
	}
	return metric
}

// AsrMetric extends Metric with additional metrics specific to ASR requests
type AsrMetric struct {
	Metric

	LanguageTopic   string  `json:"language_topic,omitempty"`
	AudioDurationMs []int64 `json:"audio_duration_ms,omitempty"`
	AsrLatencyMs    []int64 `json:"asr_latency_ms,omitempty"`
	AvgPacketSize   []int64 `json:"avg_packet_size,omitempty"`
	// AudioStreamingRate []float64 `json:"audio_streaming_rate,omitempty"`
	CompletionCause []string `json:"completion_cause,omitempty"`
}

// calcAsrLatency() calculates recognition latency based on distance between last audio packet received to recognition completion
func (a *AsrMetric) calcAsrLatency(record tp.EventLogSchema) (latencies []int64) {

	latencies = make([]int64, len(record.Value.Data.CallSummary.Recognize))

	// For ASR requests where utterance detection type is set to MULTIPLE, we could have multiple latencies to calculate...
	for idx, obj := range record.Value.Data.CallSummary.Recognize {
		if lastPacketTime, found := obj.(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["lastPacketTime"].(string); found {
			absEndTime := int64(obj.(tp.JsonObj)["absEndTime"].(float64))
			dt, _ := time.Parse(time.RFC3339, lastPacketTime)
			absLastPacketTime := dt.UnixNano() / 1000000
			latencies[idx] = absEndTime - absLastPacketTime
		}
	}
	return latencies
}

// calcAvgPacketSize() calculates average packet size based on total bytes received deviced by total number of audio packets received
func (a *AsrMetric) calcAvgPacketSize(record tp.EventLogSchema) (avgPacketSizes []int64) {
	avgPacketSizes = make([]int64, len(record.Value.Data.CallSummary.Recognize))

	// For ASR requests where utterance detection type is set to MULTIPLE, we could have multiple avg packet sizes to calculate...
	for idx, obj := range record.Value.Data.CallSummary.Recognize {
		if packetsReceived, found := obj.(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["packetsReceived"].(float64); found {
			if packetsReceived > 0 {
				bytesReceived := int64(obj.(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["bytesReceived"].(float64))
				avgPacketSizes[idx] = (bytesReceived / int64(packetsReceived))
			} else {
				avgPacketSizes[idx] = 0
			}
		} else {
			avgPacketSizes[idx] = 0
		}
	}

	return avgPacketSizes
}

// calcAudioDuration() extracts the audio duration measure available in the asr-callsummary log
func (a *AsrMetric) calcAudioDuration(record tp.EventLogSchema) (audioDurations []int64) {
	audioDurations = make([]int64, len(record.Value.Data.CallSummary.Recognize))

	// For ASR requests where utterance detection type is set to MULTIPLE, we could have multiple audio durations to extract...
	for idx, obj := range record.Value.Data.CallSummary.Recognize {
		if audioDurationMs, found := obj.(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["audioDurationMs"].(float64); found {
			audioDurations[idx] = int64(audioDurationMs)
		} else {
			audioDurations[idx] = 0
		}
	}

	return audioDurations
}

// calcCompletionCause() extracts the recognition completion cause from the asr-callsummary log
func (a *AsrMetric) calcCompletionCause(record tp.EventLogSchema) (causes []string) {
	causes = make([]string, len(record.Value.Data.CallSummary.Recognize))

	// For ASR requests where utterance detection type is set to MULTIPLE, we could have multiple completion causes to extract...
	for idx, obj := range record.Value.Data.CallSummary.Recognize {
		utterances := obj.(tp.JsonObj)["utterances"].(tp.JsonArr)
		if cause, found := utterances[0].(tp.JsonObj)["completionCause"]; found {
			causes[idx] = cause.(string)
		} else {
			causes[idx] = ""
		}
	}

	return causes
}

// TtsMetric extends Metric with additional metrics specific to TTS requests
type TtsMetric struct {
	Metric
	Voice string `json:"voice,omitempty"`
}

// NluMetric extends Metric with additional metrics specific to NLU requests
type NluMetric struct {
	Metric
}

// DlgMetric extends Metric with additional metrics specific to Dialog requests
type DlgMetric struct {
	Metric
	DialogRPC string `json:"dialog_rpc,omitempty"`
}

// Transformer transforms event log records into metrics.
type Transformer struct {
	metric interface{}
}

//// Public methods

// NewTransformer() creates a new instance of Transformer
func NewTransformer() *Transformer {
	return &Transformer{}
}

// GetMetric() returns the metric record created by the transformer
func (t *Transformer) GetMetric() interface{} {
	return t.metric
}

//// Helpers...

// isCallSummary() returns true if the event log data content type is a callsummary
func (t *Transformer) isCallSummary(record tp.EventLogSchema) bool {
	return strings.Contains(record.Value.Data.DataContentType, "callsummary")
}

// isCallSummaryMerged() returns true if the asr-callsummary event log has been fully merged
// with other asr event logs (e.g. asr-recognitioninitmessage, asr-finalstatus, etc.)
func (t *Transformer) isCallSummaryMerged(eventLog tp.EventLogSchema) bool {
	switch {
	case eventLog.Value.Data.Response == nil:
		return false
	case eventLog.Value.Data.Response.Status == nil:
		return false
	case eventLog.Value.Data.Request == nil:
		return false
	case eventLog.Value.Data.Request.RecognitionInitMessage == nil:
		return false
	default:
		return true
	}
}

// isTargetDlgEventType() returns true if the event log type is that of a dialog run-time request,
// essentially ignoring NII event logs
func (t *Transformer) isTargetDlgEventType(eventLog tp.EventLogSchema) bool {
	switch eventLog.Value.Type {
	case "Start", "Status", "Update", "Execute", "ExecuteStream", "Stop":
		eventName := eventLog.Value.Data.Events[0].(tp.JsonObj)["event"].(string)
		return strings.Contains(eventName, fmt.Sprintf("DLGaaS-%s-End", eventLog.Value.Type))
	default:
		return false
	}
}

//// Create New Metrics...

// newAsrMetric() creates a new instance of an ASRMetric
func (t *Transformer) newAsrMetric(eventLog tp.EventLogSchema) *AsrMetric {
	metric := &AsrMetric{
		Metric:        NewMetric(eventLog),
		LanguageTopic: eventLog.Value.Data.Request.RecognitionInitMessage["parameters"].(tp.JsonObj)["topic"].(string),
	}
	metric.AsrLatencyMs = metric.calcAsrLatency(eventLog)
	metric.AvgPacketSize = metric.calcAvgPacketSize(eventLog)
	metric.AudioDurationMs = metric.calcAudioDuration(eventLog)
	metric.CompletionCause = metric.calcCompletionCause(eventLog)
	metric.LanguageCode = eventLog.Value.Data.Request.RecognitionInitMessage["parameters"].(tp.JsonObj)["language"].(string)
	metric.ClientData = eventLog.Value.Data.CallSummary.ClientData

	return metric
}

// newTtsMetric() creates a new instance of a TTSMetric
func (t *Transformer) newTtsMetric(eventLog tp.EventLogSchema) *TtsMetric {
	metric := &TtsMetric{
		Metric: NewMetric(eventLog),
		Voice:  eventLog.Value.Data.Response.Events[0].(tp.JsonObj)["VOIC"].(string),
	}

	metric.ClientData = eventLog.Value.Data.Request.ClientData
	metric.LanguageCode = eventLog.Value.Data.Locale

	return metric
}

// newNluMetric() creates a new instance of an NLUMetric
func (t *Transformer) newNluMetric(eventLog tp.EventLogSchema) *NluMetric {
	metric := &NluMetric{
		Metric: NewMetric(eventLog),
	}

	metric.ClientData = eventLog.Value.Data.Request.ClientData
	metric.LanguageCode = eventLog.Value.Data.Locale

	return metric
}

// newDlgMetric() creates a new instance of an DLGMetric
func (t *Transformer) newDlgMetric(eventLog tp.EventLogSchema) *DlgMetric {
	metric := &DlgMetric{
		Metric:    NewMetric(eventLog),
		DialogRPC: eventLog.Value.Type,
	}

	metric.LanguageCode = eventLog.Value.Data.Request.Selector["language"].(string)

	return metric
}

//// Check Record Type...

// isAsraasRecord() returns true if the event log was generated by ASR
func (t *Transformer) isAsraasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "ASRaaS"
}

// isTtsaasRecord returns true if the event log was generated by TTS
func (t *Transformer) isTtsaasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "TTSaaS"
}

// isNluaasRecord() returns true if the event log was generated by NLU
func (t *Transformer) isNluaasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "NLUaaS"
}

// isDlgaasRecord() returns true if the event log was generated by Dialog
func (t *Transformer) isDlgaasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "DLGaaS"
}

// Transform applies event log data transformation
func (t *Transformer) Transform(record tp.EventLogSchema) (tp.EventLogSchema, error) {
	switch {
	case t.isDlgaasRecord(record) && t.isTargetDlgEventType(record):
		t.metric = t.newDlgMetric(record)
		return record, nil
	case t.isNluaasRecord(record):
		t.metric = t.newNluMetric(record)
		return record, nil
	case t.isTtsaasRecord(record):
		t.metric = t.newTtsMetric(record)
		return record, nil
	case t.isAsraasRecord(record) && t.isCallSummary(record) && t.isCallSummaryMerged(record):
		t.metric = t.newAsrMetric(record)
		return record, nil
	default:
		return record, errors.New("invalid record type")
	}
}
