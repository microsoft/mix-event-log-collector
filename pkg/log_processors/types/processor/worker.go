package processor

/*
	Base worker implementation for custom workers to inherit from
*/

import (
	"encoding/json"

	"github.com/google/uuid"
	pt "nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	t "nuance.xaas-logging.event-log-collector/pkg/types"
)

// Worker does the actual processing of an event log. It maintains all current state of the
// event log as it passes thru each processing step.
type Worker struct {
	ID       uuid.UUID
	Record   []byte
	Schema   t.EventLogSchema
	Failed   bool
	Valid    bool
	Filtered bool
	Written  bool
	Err      error
	Filter_  Filter
}

// NewWorker() creates an instance of a base implementatino of LogProcessorWorker
func NewWorker(config Config) pt.LogProcessorWorker {
	id, _ := uuid.NewUUID()
	return &Worker{
		ID:       id,
		Record:   nil,
		Failed:   false,
		Filtered: false,
		Err:      nil,
		Filter_:  *NewFilter(config.FilterConfig),
	}
}

// EventLog() returns the available EventLogSchema
func (w *Worker) EventLog() t.EventLogSchema {
	return w.Schema
}

// IsValid() returns true if the event log record passed Validation()
func (w *Worker) IsValid() bool {
	return w.Valid
}

// IsFailed() returns true if processing of the event log record has failed
func (w *Worker) IsFailed() bool {
	return w.Failed
}

// IsWritten() returns true if the processed event log has been successfully written out to the data pipeline
func (w *Worker) IsWritten() bool {
	return w.Written
}

// IsFiltered() returns true if the event log record has been filtered from the data pipeline
func (w *Worker) IsFiltered() bool {
	return w.Filtered
}

// LastError() returns the last error encountered while processing the event log record
func (w *Worker) LastError() error {
	return w.Err
}

// Start() initializes the worker in preparation of processing a record
func (w *Worker) Start(record []byte) pt.LogProcessorWorker {
	w.Record = record
	w.Failed = false
	w.Valid = false
	w.Filtered = false
	w.Written = false
	w.Err = nil

	log.Debugf("[worker id: %v] worker started", w.ID.String())
	return w
}

// Validate() verifies the event log record can be marshalled into a valid EventLogSchema
func (w *Worker) Validate() pt.LogProcessorWorker {
	if w.Failed {
		return w
	}

	err := json.Unmarshal(w.Record, &w.Schema)
	if err != nil {
		w.Err = err
		w.Failed = true
		log.Debugf("[worker id: %v] log validation: FAILED", w.ID.String())
		log.Errorf("[worker id: %v] %v", w.ID.String(), err)
	}
	w.Valid = true

	log.Debugf("[worker id: %v] log validation: PASSED", w.ID.String())
	return w
}

// Filter() applies filtering rules on the event log record. If filtered, all downstream processing
// methods are ignored and the event log record is removed from the data pipeline
func (w *Worker) Filter() pt.LogProcessorWorker {
	if w.Failed {
		return w
	}

	if w.Filter_.IsFiltered(w.Schema.Value.Data.DataContentType) {
		log.Debugf("[worker id: %v] log filter: FAILED", w.ID.String())
		w.Failed = true
		w.Filtered = true
	} else {
		log.Debugf("[worker id: %v] log filter: PASSED", w.ID.String())
	}
	return w
}

// Transform() applies transformation rules to the event log record. The base worker does not apply
// any transformations. Custom workers that extend this base worker should create their own transformer
func (w *Worker) Transform() pt.LogProcessorWorker {
	return w
}

// Join() applies event log merging rules to two or more event log records. A cache is required to join records
// and custom workers that extend this base worker should create their own Join logic. The base worker does not apply
// any joins
func (w *Worker) Join() pt.LogProcessorWorker {
	return w
}

// Write() writes the processed event log record back into the data pipeline if it has not been filtered
func (w *Worker) Write(writer io.Writer) pt.LogProcessorWorker {
	if w.Failed {
		return w
	}
	bytes, _ := json.Marshal(w.Schema)
	err := writer.Write(bytes)
	if err != nil {
		w.Err = err
		w.Failed = true
		log.Errorf("[worker id: %v] %v", w.ID.String(), err)
	} else {
		w.Written = true
		log.Debugf("[worker id: %v] log write: PASSED", w.ID.String())
	}

	return w
}
