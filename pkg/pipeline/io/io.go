package io

import (
	"errors"
	"fmt"

	p "nuance.xaas-logging.event-log-collector/pkg/pipeline"
	e "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/provider"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
)

const (
	ErrLoadConfig         = "could not load pipeline config: %v"
	ErrWriterNotSupported = "error - pipeline writer is not supported with the 'writer' component"
	ErrReaderNotSupported = "error - pipeline reader is not supported with the 'fetcher' component"
)

// CreateWrite() creates an instance of a pipeline writer for the target pipeline component
func CreateWriter(component p.PipelineComponent, configFile string) (io.Writer, error) {

	pipeline, err := p.LoadPipelineConfigFromFile(configFile)
	if err != nil {
		return nil, fmt.Errorf(ErrLoadConfig, err)
	}
	switch component {
	case p.FETCHER:
		return e.NewWriter(pipeline.Pipeline.StorageProvider, *pipeline.Pipeline.Fetcher.Writer), nil
	case p.PROCESSOR:
		return e.NewWriter(pipeline.Pipeline.StorageProvider, *pipeline.Pipeline.Processor.Writer), nil
	case p.WRITER:
		return nil, errors.New(ErrWriterNotSupported)
	}
	return nil, fmt.Errorf(ErrLoadConfig, err)
}

// CreateReader() creates an instance of a pipeline reader for the target pipeline component
func CreateReader(component p.PipelineComponent, configFile string) (io.Reader, error) {
	pipeline, err := p.LoadPipelineConfigFromFile(configFile)
	if err != nil {
		return nil, fmt.Errorf(ErrLoadConfig, err)
	}
	switch component {
	case p.FETCHER:
		return nil, errors.New(ErrReaderNotSupported)
	case p.PROCESSOR:
		return e.NewReader(pipeline.Pipeline.StorageProvider, *pipeline.Pipeline.Processor.Reader), nil
	case p.WRITER:
		return e.NewReader(pipeline.Pipeline.StorageProvider, *pipeline.Pipeline.Writer.Reader), nil
	}
	return nil, fmt.Errorf(ErrLoadConfig, err)
}
