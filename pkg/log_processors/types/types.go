package types

import (
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/types"
)

// TODO: this is a placeholder for providing clients a way to get a processors's status
type LogProcessorStatus struct{}

// function definitions to create and run custom process workers
type WorkerFactory = func() LogProcessorWorker
type RunWorkerFunc = func(worker LogProcessorWorker, record []byte)

type LogProcessor interface {
	Start() error
	Stop()
	GetStatus() (LogProcessorStatus, error)
}

// LogProcessorWorker interface defines a few typical data pipeline processing
// methods like Filter(), Transform(), and Join()
type LogProcessorWorker interface {
	// lifecycle....
	Start([]byte) LogProcessorWorker
	Validate() LogProcessorWorker
	Filter() LogProcessorWorker
	Transform() LogProcessorWorker
	Join() LogProcessorWorker
	Write(io.Writer) LogProcessorWorker

	// State...
	IsFailed() bool
	IsValid() bool
	IsFiltered() bool
	IsWritten() bool
	LastError() error
	EventLog() types.EventLogSchema
}

// Factory method definition
type LogProcessorFactory func(configFile string) LogProcessor
