package redis

/*
 * redis reader reads from a redis pub/sub channel.
 */

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	extc "github.com/reugn/go-streams/extension"
	extr "github.com/reugn/go-streams/redis"

	"github.com/reugn/go-streams/flow"
	"nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/utils"
)

type Reader struct {
	//path string

	source  *extr.PubSubSource
	sink    *extc.ChanSink
	mapFlow *flow.PassThrough
	out     chan interface{}
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewReader(config pipeline.ReaderParams) io.Reader {
	reader := Reader{
		mapFlow: flow.NewPassThrough(),
		out:     make(chan interface{}),
	}

	reader.ctx, reader.cancel = context.WithCancel(context.Background())
	reader.sink = extc.NewChanSink(reader.out)

	options := &redis.Options{
		Addr:     *config.Host,    // use default Addr
		Password: config.Password, // no password set
		DB:       config.DB,       // use default DB
	}
	logging.Debugf("redis options: %+v", *options)

	var err error
	reader.source, err = extr.NewPubSubSource(reader.ctx, redis.NewClient(options), config.Channel)
	if err != nil {
		logging.Errorf("%v", err)
		return nil
	}
	logging.Debugf("redis source: %+v", *reader.source)

	return &reader
}

func (w *Reader) Open() (err error) {
	go w.source.
		Via(w.mapFlow).
		To(w.sink)
	return nil
}

func (w *Reader) Close() {
	w.ctx.Done()
	logging.Debugf("redis reader closed")
}

func (w *Reader) Read() ([]byte, error) {
	data := <-w.out
	logging.Debugf("gostreamer read()  --> %+v", data)
	switch data.(type) {
	case nil:
		return nil, errors.New("reader closed")
	case *redis.Message:
		payload := data.(*redis.Message).Payload
		logging.Debugf("gostreamer read() payload  --> %+v", payload)
		record, err := utils.PreProcessRedactedFields([]byte(payload))
		if err != nil {
			return []byte(payload), fmt.Errorf(io.InvalidPayloadError, err, payload)
		}
		return record, nil
	default:
		return nil, errors.New("unknown payload type")
	}
}
