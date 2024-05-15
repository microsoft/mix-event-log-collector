package mix_bi

/*
 * mix-bi log processor transforms the event log into a smaller data object containing a handful of relevant
 * KPI's for each service type (asr, nlu, tts, dlg). This is useful when business analytics are of
 * primary concern and data storage is limited
 */

import (
	pt "nuance.xaas-logging.event-log-collector/pkg/log_processors/types"
	p "nuance.xaas-logging.event-log-collector/pkg/log_processors/types/processor"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type LogProcessor struct {
	p.LogProcessor
}

// NewLogProcessor() creates a new instance of a mix-bi LogProcessor
func NewLogProcessor(configFile string) pt.LogProcessor {
	log.Debugf("factory creating new mix-bi processor")

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

// NewWorker() creates a mix-bi custom worker
func (p *LogProcessor) NewWorker() pt.LogProcessorWorker {
	log.Debugf("instantiating %v worker", p.Name)
	return NewWorker(p.Cfg)
}

// RunWorker() implements the custom mix-bi worker flow
func (p *LogProcessor) RunWorker(worker pt.LogProcessorWorker, record []byte) {
	log.Debugf("running %v worker", p.Name)

	// Note the Join() comes before Transform() which is opposite of mix-default...
	worker.Start(record).Validate().Filter().Join().Transform().Write(p.Writer)
}
