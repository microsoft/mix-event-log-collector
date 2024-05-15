package mix_bi

/*
 * Mix-bi worker extends the base worker to merge asr event logs and apply custom transformations
 */

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	c "nuance.xaas-logging.event-log-collector/pkg/cache"
	pt "nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	p "nuance.xaas-logging.event-log-collector/pkg/log_processors/types/processor"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	t "nuance.xaas-logging.event-log-collector/pkg/types"
)

const (
	// asr event log types that need to be merged...
	KeyRecognitionInitMessage = "recognitioninitmessage"
	KeyFinalStatusResponse    = "finalstatusresponse"
	KeyFinalResultResponse    = "finalresultresponse"
	KeyCallSummary            = "callsummary"

	ErrAsrKeyNotFound = "asr key unexpectedly not found: %v"
)

// Worker extends base worker with custom call log merging and transformations
type Worker struct {
	parent *p.Worker

	transformer       Transformer
	cache             c.Cache
	keys              []string
	joinedCallSummary *t.EventLogSchema
}

// NewWorker() creates a new instance of a mix-bi worker
func NewWorker(config p.Config) pt.LogProcessorWorker {
	return &Worker{
		parent:            p.NewWorker(config).(*p.Worker),
		transformer:       *NewTransformer(),
		keys:              []string{KeyCallSummary, KeyFinalResultResponse, KeyFinalStatusResponse, KeyRecognitionInitMessage},
		joinedCallSummary: nil,
		cache:             config.Cache,
	}
}

// Parent Worker Implementation...

func (w *Worker) EventLog() t.EventLogSchema {
	return w.parent.EventLog()
}

func (w *Worker) IsValid() bool {
	return w.parent.IsValid()
}

func (w *Worker) IsFailed() bool {
	return w.parent.IsFailed()
}

func (w *Worker) IsWritten() bool {
	return w.parent.IsWritten()
}

func (w *Worker) IsFiltered() bool {
	return w.parent.IsFiltered()
}

func (w *Worker) LastError() error {
	return w.parent.LastError()
}

func (w *Worker) Start(record []byte) pt.LogProcessorWorker {
	w.parent.Start(record)

	// For process chaining to behave as expected,
	// we need to make sure we return this worker instance, not the base worker
	return w
}

func (w *Worker) Validate() pt.LogProcessorWorker {
	w.parent.Validate()

	// For process chaining to behave as expected,
	// we need to make sure we return this worker instance, not the base worker
	return w
}

func (w *Worker) Filter() pt.LogProcessorWorker {
	w.parent.Filter()

	// If this is an asr event log and meets certain criteria, cache it for eventual merging
	if w.useAsrRecordCache() {
		w.cacheAsrRecord()
	}

	// For process chaining to behave as expected,
	// we need to make sure we return this worker instance, not the base worker
	return w
}

// Mix-bi worker overrides...

// Transform() applies transformations to the event log record
func (w *Worker) Transform() pt.LogProcessorWorker {
	// Cached asr records still need to be transformed, even if there's been a failure (e.g. filtered)
	if w.parent.Failed && !w.useAsrRecordCache() {
		return w
	}

	var err error

	// Apply some custom decision logic on when/how to apply transformations
	switch {
	case w.joinedCallSummary != nil:
		// If we have an asr callsummary log that's been fully merged, pass this joined event log to the transformer
		_, err = w.transformer.Transform(*w.joinedCallSummary)
	case w.useAsrRecordCache() && w.joinedCallSummary == nil:
		// We can't do transformation on asr callsummary log until it's been fully merged
		err = errors.New("call summary has not been joined yet")
	default:
		// For all non-asr event logs, pass in the default validated event log
		w.parent.Schema, err = w.transformer.Transform(w.parent.Schema)
	}

	// error check
	if err != nil {
		log.Debugf("[worker id: %v] log transformer: FAILED", w.parent.ID.String())
		w.parent.Err = err
		w.parent.Failed = true
	} else {
		log.Debugf("[worker id: %v] log transformation: PASSED", w.parent.ID.String())
	}

	return w
}

// Join() attempts to merge a number of asr event logs into the asr-callsummary. Event logs are cached
// until all are available to be merged
func (w *Worker) Join() pt.LogProcessorWorker {
	// asr-callsummary event logs are not being filtered and all asr event logs we need are present
	if w.shouldJoin() {
		w.joinAsrRecords()
	}

	if !w.parent.Failed {
		log.Debugf("[worker id: %v] log join: PASSED", w.parent.ID.String())
	}

	return w
}

// Write() writes the metric to the data pipeline
func (w *Worker) Write(writer io.Writer) pt.LogProcessorWorker {
	if w.parent.Failed {
		return w
	}

	// We write the metric, not the event log...
	bytes, _ := json.Marshal(w.transformer.GetMetric())

	err := writer.Write(bytes)
	if err != nil {
		w.parent.Err = err
		w.parent.Failed = true
		log.Errorf("[worker id: %v] %v", w.parent.ID.String(), err)
	} else {
		w.parent.Written = true
		log.Debugf("[worker id: %v] log write: PASSED", w.parent.ID.String())
	}

	return w
}

// Helpers...

// useAsrRecordCache() checks if the record was generated by the ASR service and
// applies some additional criteria to detemrine if it should be cached for merging
func (w *Worker) useAsrRecordCache() bool {
	return (w.parent.Schema.Key.Service == "ASRaaS" &&
		!w.parent.Filter_.IsAsrSummaryIgnored() &&
		len(w.parseAsrEventLogName()) > 0)
}

// parseAsrEventLogName() parses the event log name if it matches one of the names defined under consts
func (w *Worker) parseAsrEventLogName() string {
	switch {
	case strings.Contains(w.parent.Schema.Value.Data.DataContentType, KeyRecognitionInitMessage):
		return KeyRecognitionInitMessage
	case strings.Contains(w.parent.Schema.Value.Data.DataContentType, KeyFinalResultResponse):
		return KeyFinalResultResponse
	case strings.Contains(w.parent.Schema.Value.Data.DataContentType, KeyFinalStatusResponse):
		return KeyFinalStatusResponse
	case strings.Contains(w.parent.Schema.Value.Data.DataContentType, KeyCallSummary):
		return KeyCallSummary
	default:
		return ""
	}
}

// createCacheKey() generates a key name based on ASR session id to use with the cache
func (w *Worker) createCacheKey(eventLogName string) string {
	return fmt.Sprintf("%s|%s",
		w.parent.Schema.Value.Data.AsrSessionID,
		eventLogName)
}

// cacheAsrRecord() caches ASR records if they pass decision logic
func (w *Worker) cacheAsrRecord() {
	if w.useAsrRecordCache() {
		log.Debugf("[worker id: %v] CACHING RECORD: %v [asr session id = %v",
			w.parent.ID.String(),
			w.parent.Schema.Value.Data.DataContentType,
			w.parent.Schema.Value.Data.AsrSessionID)
		key := w.createCacheKey(w.parseAsrEventLogName())
		w.cache.Set(key, w.parent.Schema, 60*time.Minute)
	}
}

// allAsrRecordsAvailableForJoin() checks the cache to see if all asr records are now available to be joined
func (w *Worker) allAsrRecordsAvailableForJoin() bool {
	for _, k := range w.keys {
		var s t.EventLogSchema
		if !w.cache.Get(w.createCacheKey(k), &s) {
			log.Debugf("[worker id: %v] Missing ASR event log: %v", w.parent.ID.String(), k)
			return false
		}
		log.Debugf("[worker id: %v] CACHED RECORD FOUND: %v", w.parent.ID.String(), s.Value.Data.DataContentType)
	}
	return true
}

// shouldJoin() returns true if criteria are met for merging ASR event logs
func (w *Worker) shouldJoin() bool {
	return w.useAsrRecordCache() && w.allAsrRecordsAvailableForJoin()
}

// deleteKeysFromCache() deletes all cached ASR event logs for this worker's ASR Session ID
func (w *Worker) deleteKeysFromCache() {
	for _, k := range w.keys {
		w.cache.Delete(w.createCacheKey(k))
	}
}

// getAsrEventLogsFromCahce() gets all ASR event logs available in the cache. If any are missing an error is returned
func (w *Worker) getAsrEventLogsFromCache(recoInit, finalResult, finalStatus, callSummary *t.EventLogSchema) (err error) {
	// Get the asr event logs from cache
	for _, k := range w.keys {
		switch k {
		case KeyRecognitionInitMessage:
			if !w.cache.Get(w.createCacheKey(k), &recoInit) {
				err = fmt.Errorf(ErrAsrKeyNotFound, k)
			}
		case KeyFinalResultResponse:
			if !w.cache.Get(w.createCacheKey(k), &finalResult) {
				err = fmt.Errorf(ErrAsrKeyNotFound, k)
			}
		case KeyFinalStatusResponse:
			if !w.cache.Get(w.createCacheKey(k), &finalStatus) {
				err = fmt.Errorf(ErrAsrKeyNotFound, k)
			}
		case KeyCallSummary:
			if !w.cache.Get(w.createCacheKey(k), &callSummary) {
				err = fmt.Errorf(ErrAsrKeyNotFound, k)
			}
		}
	}

	return err
}

// mergeIntoAsrCallSummary takes data from related ASR event logs and merges some of their data into the ASR CallSummary log
func (w *Worker) mergeIntoAsrCallSummary(recoInit, finalResult, finalStatus, callSummary *t.EventLogSchema) {
	// merge records into call summary
	log.Debugf("[worker id: %v] merging asr call logs for asr session id %v", w.parent.ID.String(), callSummary.Value.Data.AsrSessionID)

	// reco init
	callSummary.Value.Data.Request = recoInit.Value.Data.Request

	// final status
	callSummary.Value.Data.Response = finalStatus.Value.Data.Response
	dt, _ := time.Parse(time.RFC3339, finalStatus.Value.Timestamp)
	callSummary.Value.Data.Response.Timestamp = dt.UnixNano() / 1000000

	// final result
	dt, _ = time.Parse(time.RFC3339, finalResult.Value.Timestamp)
	callSummary.Value.Data.FinalResult = &t.AsrFinalResultPayload{
		Timestamp: dt.UnixNano() / 1000000,
	}

	// If we're operating on an asr-callsummary record, update the current schema with the
	// newly merged version of asr-callsummary. If event logs were received/cached out of order
	// set joinedCallSummary to the newly merged version of asr-callsummary in order that
	// Write() knows that two event logs need to be written to the data pipeline
	if w.parent.Schema.Value.Data.DataContentType == callSummary.Value.Data.DataContentType {
		w.parent.Schema = *callSummary
	} else {
		w.joinedCallSummary = callSummary
	}
}

// joinAsrRecords() wraps functionality associated with merging ASR call log data into the ASR call summary event log.
// This method should only be called if shouldJoin() has been called and returned true
func (w *Worker) joinAsrRecords() {
	var recoInit t.EventLogSchema
	var finalResult t.EventLogSchema
	var finalStatus t.EventLogSchema
	var callSummary t.EventLogSchema
	var err error

	// Remove asr event logs from cache once we're done with the join
	defer w.deleteKeysFromCache()

	// Get the asr event logs from cache
	err = w.getAsrEventLogsFromCache(&recoInit, &finalResult, &finalStatus, &callSummary)
	if err != nil {
		log.Errorf("[worker id: %v] %v [asr session id: %v]",
			w.parent.ID.String(),
			err,
			w.parent.Schema.Value.Data.AsrSessionID)
		return
	} else {
		w.mergeIntoAsrCallSummary(&recoInit, &finalResult, &finalStatus, &callSummary)
	}
}
