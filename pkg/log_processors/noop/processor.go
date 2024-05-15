package noop

/*
 * noop log processor does not do any event log transformations. It does validate the event log
 * and applies filtering rules before writing the event log as-is back out to the data pipeline
 */

import (
	pt "nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	p "nuance.xaas-logging.event-log-collector/pkg/log_processors/types/processor"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type LogProcessor struct {
	p.LogProcessor
}

// NewLogProcessor() creates a new instance of a noop LogProcessor
func NewLogProcessor(configFile string) pt.LogProcessor {
	log.Debugf("factory creating new noop processor")

	cfg, err := p.LoadConfigFromFile(configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}

	processor := LogProcessor{
		p.LogProcessor{
			Name: cfg.Processor.Name,
			Cfg:  cfg.Processor.Config,

			Shutdown: make(chan bool),
			Records:  make(chan []byte, cfg.Processor.Config.NumWorkers),
			Job:      make(chan interface{}),
		},
	}

	// Initialize the processor
	processor.Init(configFile, cfg)

	// Apply overrides
	processor.RunWorkerFunc = processor.RunWorker

	return &processor
}

// RunWorker() validates and filters event logs, but does not do any transformations or joins
func (p *LogProcessor) RunWorker(worker pt.LogProcessorWorker, record []byte) {
	log.Debugf("running %v worker", p.Name)
	worker.Start(record).Validate().Filter().Write(p.Writer)
}
