package io

import "nuance.xaas-logging.event-log-collector/pkg/pipeline"

const (
	InvalidPayloadError = "invalid payload [err = %v]: %v"
)

// Reader defines a pipeline reader
type Reader interface {
	Open() error
	Close()
	Read() ([]byte, error)
}
type ReaderFactory func(pipeline.ReaderParams) Reader

// Writer defines a pipeline writer
type Writer interface {
	Open() error
	Close()
	Write([]byte) error
}
type WriterFactory func(pipeline.WriterParams) Writer
