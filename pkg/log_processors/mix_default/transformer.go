package mix_default

/*
 * Mix-default transformer normalizes some fields across service types and does some calculations on
 * asr event logs to make it easier to create analytics
 */

import (
	"reflect"
	"strings"
	"time"

	tp "nuance.xaas-logging.event-log-collector/pkg/types"
)

// Transformer provides some basic event log normalization and calculations (e.g. latency) that make it easier for
// downstream storage solutions (e.g. elasticsearch) to process the event log data.
type Transformer struct{}

//// Public methods

// NewTransformer() creates a new instance of Transformer
func NewTransformer() *Transformer {
	return &Transformer{}
}

//// Helpers...

// hasAsrResultForInput() checks for presence of an ASR Result (vs. text) for input to NLU
func (t *Transformer) hasAsrResultForInput(record tp.EventLogSchema) bool {
	if reflect.TypeOf(record.Value.Data.Request.Input).Kind() == reflect.String {
		return false
	}

	if _, found := record.Value.Data.Request.Input.(tp.JsonObj)["asrResult"]; found {
		return true
	}
	return false
}

// hasRedactedInput() checks NLU input for redacted text
func (t *Transformer) hasRedactedInput(record tp.EventLogSchema) bool {
	return reflect.TypeOf(record.Value.Data.Request.Input).Kind() == reflect.String
}

// hasRedactedResponse() checks NLU response for redacted results
func (t *Transformer) hasRedactedResponse(record tp.EventLogSchema) bool {
	return record.Value.Data.Response.ResultPayload != nil &&
		strings.Contains(record.Value.Data.Response.ResultPayload.Literal, "redacted")
}

// removeRedactedInput() replaces the redacted input (type: string) with a json object object in order to normalize the input field type
func (t *Transformer) removeRedactedInput(record tp.EventLogSchema) tp.EventLogSchema {
	if t.hasRedactedInput(record) {
		record.Value.Data.Request.Input = make(tp.JsonObj)
		record.Value.Data.Request.Input.(tp.JsonObj)["inputUnion"] = "text"
		record.Value.Data.Request.Input.(tp.JsonObj)["text"] = "<text input redacted>"
	}
	return record
}

// removeRedactedResponse() replaces the redacted response (type: string) with a json object object in order to normalize the result field type
func (t *Transformer) removeRedactedResponse(record tp.EventLogSchema) tp.EventLogSchema {
	if t.hasRedactedResponse(record) {
		record.Value.Data.Response.ResultPayload.Literal = "<response redacted>"
	}
	return record
}

// isCallSummary() returns true if the event log data content type is a callsummary
func (t *Transformer) isCallSummary(record tp.EventLogSchema) bool {
	return strings.Contains(record.Value.Data.DataContentType, "callsummary")
}

// isFinalStatusResponse() returns true if the event log data content type is a finalstatusresponse
func (t *Transformer) isFinalStatusResponse(record tp.EventLogSchema) bool {
	return strings.Contains(record.Value.Data.DataContentType, "finalstatusresponse")
}

// mapWeightEnumToInt() normalizes the weight value if provided in a recognitionObject for ASR
func (t *Transformer) mapWeightEnumToInt(record tp.EventLogSchema) tp.EventLogSchema {
	for idx, obj := range record.Value.Data.CallSummary.Recognize {
		if _, found := obj.(tp.JsonObj)["recognitionObjects"]; found {
			for idx2, ro := range obj.(tp.JsonObj)["recognitionObjects"].(tp.JsonArr) {
				if _, found := ro.(tp.JsonObj)["weight"]; found {
					// TODO: this is incomplete. need to map all weight enums to their numeric values
					record.Value.Data.CallSummary.Recognize[idx].(tp.JsonObj)["recognitionObjects"].(tp.JsonArr)[idx2].(tp.JsonObj)["weight"] = 500
				}
			}
		}
	}
	return record
}

// calcAsrLatency() calculates recognition latency based on distance between last audio packet received to recognition completion
func (t *Transformer) calcAsrLatency(record tp.EventLogSchema) tp.EventLogSchema {
	for idx, obj := range record.Value.Data.CallSummary.Recognize {
		if lastPacketTime, found := obj.(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["lastPacketTime"].(string); found {
			absEndTime := int64(obj.(tp.JsonObj)["absEndTime"].(float64))
			dt, _ := time.Parse(time.RFC3339, lastPacketTime)
			absLastPacketTime := dt.UnixNano() / 1000000
			latency := absEndTime - absLastPacketTime
			obj.(tp.JsonObj)["latency"] = latency
			record.Value.Data.CallSummary.Recognize[idx].(tp.JsonObj)["latency"] = latency
		}
	}
	return record
}

// calcAvgPacketSize() calculates average packet size based on total bytes received deviced by total number of audio packets received
func (t *Transformer) calcAvgPacketSize(record tp.EventLogSchema) tp.EventLogSchema {
	avgPacketSize := 0
	for idx, obj := range record.Value.Data.CallSummary.Recognize {
		if packetsReceived, found := obj.(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["packetsReceived"].(float64); found {
			if packetsReceived > 0 {
				bytesReceived := int(obj.(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["bytesReceived"].(float64))
				avgPacketSize = (bytesReceived / int(packetsReceived))
			}
			record.Value.Data.CallSummary.Recognize[idx].(tp.JsonObj)["audioPacketStats"].(tp.JsonObj)["avgPacketSize"] = avgPacketSize
		}
	}
	return record
}

// normalizeAsrClientData() moves the location of clientData in the asr callsummary event log to a common location that mirrors other callsummary event logs
// this is helpful with nosql databases like elasticsearch that are auto-indexing json objects
func (t *Transformer) normalizeAsrClientData(record tp.EventLogSchema) tp.EventLogSchema {
	record.Value.Data.ClientData = record.Value.Data.CallSummary.ClientData
	record.Value.Data.Request = &tp.RequestPayload{}
	record.Value.Data.Request.ClientData = make(tp.JsonObj)
	record.Value.Data.Request.ClientData = record.Value.Data.CallSummary.ClientData
	record.Value.Data.CallSummary.ClientData = nil

	return record
}

// normalizeTtsClientData() moves the location of clientData in the tts callsummary event log to a common location that mirrors other callsummary event logs
// this is helpful with nosql databases like elasticsearch that are auto-indexing json objects
func (t *Transformer) normalizeTtsClientData(record tp.EventLogSchema) tp.EventLogSchema {
	record.Value.Data.Request.ClientData = record.Value.Data.ClientData
	record.Value.Data.ClientData = nil
	return record
}

//// Check Record Type...

// isAsraasRecord() returns true if the event log was generated by ASR
func (t *Transformer) isAsraasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "ASRaaS"
}

// isNluaasRecord() returns true if the event log was generated by NLU
func (t *Transformer) isNluaasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "NLUaaS"
}

// isTtsaasRecord() returns true if the event log was generated by TTS
func (t *Transformer) isTtsaasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "TTSaaS"
}

// isDlgaasRecord() returns true if the event log was generated by Dialog
func (t *Transformer) isDlgaasRecord(record tp.EventLogSchema) bool {
	return record.Value.Service == "DLGaaS"
}

// isNIIEvent() returns true if the event log was generated by NII
func (t *Transformer) isNIIEvent(record tp.EventLogSchema) bool {
	return record.Value.Service == "NIIEventLog"
}

// Transform applies event log data transformation
func (t *Transformer) Transform(record tp.EventLogSchema) (tp.EventLogSchema, error) {
	switch {
	case t.isNIIEvent(record):
		return record, nil // Do nothing
	case t.isDlgaasRecord(record):
		return record, nil // Do nothing
	case t.isNluaasRecord(record):
		if t.hasAsrResultForInput(record) {
			// Remove ASR Result if provided for input because it's very large with negative impact on storage, and it's hashed which makes it useless anyway
			record.Value.Data.Request.Input.(tp.JsonObj)["asrResult"] = "<asrResult redacted>"
		} else if t.hasRedactedInput(record) {
			// When NLU input has been redacted, it's captured as a string object in the event long which
			// breaks the defined data type for this field when input is not redacted. This step helps normalize the event log field
			record = t.removeRedactedInput(record)
		}

		if t.hasRedactedResponse(record) {
			// When the NLU response has been redacted, it's captured as a string object in the event long which
			// breaks the defined data type for this field when the response is not redacted. This step helps normalize the event log field
			record = t.removeRedactedResponse(record)
		}
		return record, nil
	case t.isFinalStatusResponse(record):
		return record, nil // Do nothing
	case !t.isCallSummary(record):
		return record, nil // Do nothing
	case t.isTtsaasRecord(record):
		// Move the clientData so it's location is consistent with ASR, NLU, and Dialog event logs
		return t.normalizeTtsClientData(record), nil
	case t.isAsraasRecord(record):
		// Do a few calculations on the ASR callsummary event log
		record = t.mapWeightEnumToInt(record)
		record = t.calcAsrLatency(record)
		record = t.calcAvgPacketSize(record)
		record = t.normalizeAsrClientData(record)
		return record, nil
	default:
		return record, nil
	}
}
