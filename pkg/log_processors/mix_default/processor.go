package mix_default

/*
 * mix-default log processor transforms event logs to normalize some inconsistencies of a few event log keys
 * in where they reside across all service types (asr, nlu, tts, dlg), such as client_data. This processor
 * also merges asr event logs into a singular asr-callsummary log, as well as calculate some ASR KPI's like latency
 * to make it easier for analytics platforms to provide a consistent view of KPI's across all services.
 */

import (
	pt "nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	p "nuance.xaas-logging.event-log-collector/pkg/log_processors/types/processor"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type LogProcessor struct {
	p.LogProcessor
}

// NewLogProcessor() creates a new instance of a mix-default LogProcessor
func NewLogProcessor(configFile string) pt.LogProcessor {
	log.Debugf("factory creating new mix-default processor")

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
	processor.WorkerFactory = processor.NewWorker

	return &processor
}

// NewWorker() creates a mix-default custom worker
func (p *LogProcessor) NewWorker() pt.LogProcessorWorker {
	log.Debugf("instantiating %v worker", p.Name)
	return NewWorker(p.Cfg)
}

// RunWorker() implements the custom mix-default worker flow
func (p *LogProcessor) RunWorker(worker pt.LogProcessorWorker, record []byte) {
	log.Debugf("running %v worker", p.Name)

	// Start -> Validate -> Filter -> Transform -> Join -> Write
	worker.Start(record).Validate().Filter().Transform().Join().Write(p.Writer)
}
