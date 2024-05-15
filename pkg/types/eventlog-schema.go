package types

/*
 * EventLogSchema implements the mix event log payloads as an explicit type
 * for ASR, NLU, TTS, and Dialog event logs. This helps simplify things
 * when components need to parse and access parts of an event log rather than
 * marshaling event log payloads into generic json objects and testing/poking to see
 * if keys exist.
 *
 * Learn more about event log payloads here: https://docs.mix.nuance.com/runtime-event-logs/#top-level-kafka-structure
 */

type EventLogSchema struct {
	Topic     string        `json:"topic,omitempty"`
	Key       EventLogKey   `json:"key,omitempty"`
	Partition float64       `json:"partition,omitempty"`
	Offset    float64       `json:"offset,omitempty"`
	Value     EventLogValue `json:"value,omitempty"`
}

type EventLogKey struct {
	Service string `json:"service,omitempty"`
	ID      string `json:"id,omitempty"`
}

type EventLogValue struct {
	SpecVersion     string       `json:"specversion,omitempty"`
	Service         string       `json:"service,omitempty"`
	Source          string       `json:"source,omitempty"`
	Type            string       `json:"type,omitempty"`
	ID              string       `json:"id,omitempty"`
	Timestamp       string       `json:"timestamp,omitempty"`
	AppID           string       `json:"appid,omitempty"`
	DataContentType string       `json:"datacontenttype,omitempty"`
	Data            EventLogData `json:"data,omitempty"`
}

type EventLogData struct {
	*AsrEventLogData
	*DlgEventLogData

	DataContentType string                      `json:"dataContentType,omitempty"`
	TraceID         string                      `json:"traceid,omitempty"`
	RequestID       string                      `json:"requestid,omitempty"`
	ClientRequestID string                      `json:"clientRequestid,omitempty"`
	UserID          string                      `json:"userid,omitempty"`
	Locale          string                      `json:"locale,omitempty"`
	ClientData      JsonObj                     `json:"clientData,omitempty"`
	ProcessingTime  *EventLogDataProcessingTime `json:"processingTime,omitempty"`

	Request  *RequestPayload  `json:"request,omitempty"`
	Response *ResponsePayload `json:"response,omitempty"`
}

type EventLogDataProcessingTime struct {
	StartTime            string  `json:"startTime,omitempty"`
	FirstAudioBufferTime string  `json:"firstAudioBufferTime,omitempty"`
	DurationMs           float64 `json:"durationMs,omitempty"`
}

type DlgEventLogData struct {
	SessionID         string  `json:"sessionid,omitempty"`
	SeqID             string  `json:"seqid,omitempty"`
	Events            JsonArr `json:"events,omitempty"`
	Channel           string  `json:"channel,omitempty"`
	SessionTimeoutSec int     `json:"sessionTimeoutSec,omitempty"`
}

type AsrEventLogData struct {
	AsrSessionID    string                 `json:"asrSessionid,omitempty"`
	AudioUrn        string                 `json:"audioUrn,omitempty"`
	RedactedReason  string                 `json:"redactedReason,omitempty"`
	AudioDurationMs float64                `json:"audioDurationMs,omitempty"`
	CallSummary     *AsrCallSummaryPayload `json:"callsummary,omitempty"`
	FinalResult     *AsrFinalResultPayload `json:"finalResult,omitempty"`
}

type AsrCallSummaryPayload struct {
	SessionID          string  `json:"sessionId,omitempty"`
	ClientData         JsonObj `json:"clientData,omitempty"`
	ServerData         JsonObj `json:"serverData,omitempty"`
	AbsStartTime       int     `json:"absStartTime,omitempty"`
	DataPack           JsonObj `json:"dataPack,omitempty"`
	RecognitionObjects JsonArr `json:"recognitionObjects,omitempty"`
	Recognize          JsonArr `json:"recognize,omitempty"`
	IPKey              string  `json:"IPKey,omitempty"`
	AbsEndTime         int     `json:"absEndTime,omitempty"`
	IPS3Events         string  `json:"IP!S3Events,omitempty"`
}

type AsrFinalResultPayload struct {
	Timestamp int64 `json:"timestamp,omitempty"`
}

type RequestPayload struct {
	AsrRequestPayload
	NluRequestPayload
	DlgRequestPayload

	ClientData JsonObj `json:"clientData,omitempty"`
	Resources  JsonArr `json:"resources,omitempty"`
}

type DlgRequestPayload struct {
	SessionID         string  `json:"sessionId,omitempty"`
	Selector          JsonObj `json:"selector,omitempty"`
	Payload           JsonObj `json:"payload,omitempty"`
	SessionTimeoutSec int     `json:"sessionTimeoutSec,omitempty"`
}

type AsrRequestPayload struct {
	RecognitionInitMessage JsonObj `json:"recognitionInitMessage,omitempty"`
}

type AsrRecognitionInitMessage struct {
	Parameters JsonObj `json:"parameters,omitempty"`
	ClientData JsonObj `json:"clientData,omitempty"`
}

type NluRequestPayload struct {
	Parameters JsonObj     `json:"parameters,omitempty"`
	Model      JsonObj     `json:"model,omitempty"`
	UserID     string      `json:"userId,omitempty"`
	Input      interface{} `json:"input,omitempty"`
}

type ResponsePayload struct {
	*ResultPayload `json:"result,omitempty"`

	TtsResponsePayload
	AsrResponsePayload
	DlgResponsePayload

	Status *ResponseStatus `json:"status,omitempty"`
}

type ResponseStatus struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Details string `json:"details,omitempty"`
}

type ResultPayload struct {
	AsrResultPayload
	NluResultPayload
}

type DlgResponsePayload struct {
	Payload JsonObj `json:"event,omitempty"`
}

type TtsResponsePayload struct {
	Events JsonArr `json:"events,omitempty"`
}

type NluResultPayload struct {
	FormattedLiteral string  `json:"formattedLiteral,omitempty"`
	Interpretations  JsonArr `json:"interpretations,omitempty"`
	Literal          string  `json:"literal,omitempty"`
	Sensitive        bool    `json:"sensitive,omitempty"`
}

type AsrResultPayload struct {
	ResultType     int     `json:"resultType,omitempty"`
	AbsStartMs     int     `json:"AbsStartMs,omitempty"`
	AbsEndMs       int     `json:"AbsEndMs,omitempty"`
	DataPack       JsonObj `json:"dataPack,omitempty"`
	UtteranceInfo  JsonObj `json:"utteranceInfo,omitempty"`
	Hypotheses     JsonArr `json:"hypotheses,omitempty"`
	RedactedReason string  `json:"redactedReason,omitempty"`
}

type AsrResponsePayload struct {
	StartOfSeech *AsrStartOfSpeechPayload `json:"startOfSpeech,omitempty"`
	Timestamp    int64                    `json:"timestamp,omitempty"`
}

type AsrStartOfSpeechPayload struct {
	FirstAudioToStartOfSpeech int `json:"firstAudioToStartOfSpeech,omitempty"`
}
