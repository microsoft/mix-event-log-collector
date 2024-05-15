package kafka

/*
 * kafka writer writes to a kafka topic.
 */

import (
	extc "github.com/reugn/go-streams/extension"
	extk "github.com/reugn/go-streams/kafka"

	"github.com/IBM/sarama"
	"github.com/reugn/go-streams/flow"

	"nuance.xaas-logging.event-log-collector/pkg/logging"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
)

type Writer struct {
	config  *sarama.Config
	source  *extc.ChanSource
	mapFlow *flow.PassThrough
	sink    *extk.KafkaSink
	in      chan interface{}
}

func NewWriter(config pipeline.WriterParams) io.Writer {
	writer := Writer{
		mapFlow: flow.NewPassThrough(),
		in:      make(chan interface{}),
	}

	writer.config = sarama.NewConfig()
	writer.config.Producer.Return.Successes = true
	writer.config.Version, _ = sarama.ParseKafkaVersion("2.8.1")

	writer.source = extc.NewChanSource(writer.in)

	var err error
	writer.sink, err = extk.NewKafkaSink(config.Hosts, writer.config, *config.Topic)
	if err != nil {
		logging.Errorf("%v", err)
	}

	return &writer
}

func (w *Writer) Open() (err error) {
	go w.source.
		Via(w.mapFlow).
		To(w.sink)
	return nil
}

func (w *Writer) Close() {
	// Nothing to do...
}

func (w *Writer) enqueue(data []byte) {
	w.in <- string(data)
}

func (w *Writer) Write(data []byte) error {
	log.Debugf("writing %d bytes of data to queue", len(data))
	w.enqueue(data)
	return nil
}
