package pulsar

/*
 * pulsar writer writes to a pulsar topic.
 */

import (
	"context"
	"fmt"
	"time"

	extc "github.com/reugn/go-streams/extension"
	extk "github.com/reugn/go-streams/pulsar"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/reugn/go-streams/flow"

	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
)

type Writer struct {
	//path string

	clientOptions   *pulsar.ClientOptions
	producerOptions *pulsar.ProducerOptions
	source          *extc.ChanSource  // := ext.NewChanSource(tickerChan(time.Second * 1))
	mapFlow         *flow.PassThrough // := flow.NewPassThrough()
	sink            *extk.PulsarSink  // := ext.NewStdoutSink()
	in              chan interface{}
}

func NewWriter(config pipeline.WriterParams) io.Writer {

	if config.Host == nil {
		log.Errorf("error: pulsar host cannot be nil")
		return nil
	}

	writer := Writer{
		clientOptions:   &pulsar.ClientOptions{URL: fmt.Sprintf("pulsar://%s", *config.Host), OperationTimeout: time.Duration(config.OperationTimeout) * time.Second},
		producerOptions: &pulsar.ProducerOptions{Topic: *config.Topic},
		mapFlow:         flow.NewPassThrough(),
		in:              make(chan interface{}),
	}
	writer.source = extc.NewChanSource(writer.in)

	var err error
	writer.sink, err = extk.NewPulsarSink(context.Background(),
		writer.clientOptions,
		writer.producerOptions)
	if err != nil {
		log.Errorf("%v", err)
		return nil
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
	log.Infof("writing %d bytes of data to queue", len(data))
	w.enqueue(data)
	return nil
}
