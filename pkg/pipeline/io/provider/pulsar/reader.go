package pulsar

/*
 * pulsar reader reads from a pulsar topic.
 */

import (
	"context"
	"fmt"
	"time"

	extc "github.com/reugn/go-streams/extension"
	extk "github.com/reugn/go-streams/pulsar"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/reugn/go-streams/flow"
	"nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/utils"
)

type Reader struct {
	//path string

	clientOptions   *pulsar.ClientOptions
	consumerOptions *pulsar.ConsumerOptions
	source          *extk.PulsarSource
	sink            *extc.ChanSink
	mapFlow         *flow.PassThrough
	out             chan interface{}
	ctx             context.Context
}

func NewReader(config pipeline.ReaderParams) io.Reader {

	if config.Host == nil {
		logging.Errorf("error: pulsar host cannot be nil")
		return nil
	}

	options := &pulsar.ConsumerOptions{
		Topic:            *config.Topic,
		SubscriptionName: *config.Group,
		Type:             pulsar.Exclusive,
	}

	switch config.Offset {
	case "earliest":
		options.SubscriptionInitialPosition = pulsar.SubscriptionPositionEarliest
	case "latest":
		options.SubscriptionInitialPosition = pulsar.SubscriptionPositionLatest
	default:
		// none
	}

	reader := Reader{
		mapFlow:         flow.NewPassThrough(),
		clientOptions:   &pulsar.ClientOptions{URL: fmt.Sprintf("pulsar://%s", *config.Host), OperationTimeout: time.Duration(config.OperationTimeout) * time.Second},
		consumerOptions: options,
		out:             make(chan interface{}),
		ctx:             context.Background(),
	}

	reader.sink = extc.NewChanSink(reader.out)

	var err error
	reader.source, err = extk.NewPulsarSource(reader.ctx, reader.clientOptions, reader.consumerOptions)
	if err != nil {
		logging.Errorf("%v", err)
		return nil
	}

	return &reader
}

func (w *Reader) Open() (err error) {
	//throttler := flow.NewThrottler(1, time.Second, 50, flow.Discard)
	// slidingWindow := flow.NewSlidingWindow(time.Second*30, time.Second*5)
	//tumblingWindow := flow.NewTumblingWindow(time.Second * 5)
	go w.source.
		Via(w.mapFlow).
		//Via(throttler).
		//Via(tumblingWindow).
		To(w.sink)
	return nil
}

func (w *Reader) Close() {
	w.ctx.Done()
	logging.Debugf("pulsar reader closed")
}

func (w *Reader) Read() ([]byte, error) {
	data := <-w.out
	logging.Debugf("gostreamer read()  --> %+v", data)
	msg := data.(pulsar.Message)
	logging.Debugf("gostreamer read() payload  --> %+v", string(msg.Payload()))
	record, err := utils.PreProcessRedactedFields(msg.Payload())
	if err != nil {
		return msg.Payload(), fmt.Errorf(io.InvalidPayloadError, err, string(msg.Payload()))
	}
	return record, nil
}
