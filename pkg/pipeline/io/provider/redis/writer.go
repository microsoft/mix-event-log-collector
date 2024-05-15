package redis

/*
 * redis writer writes to a redis pub/sub channel.
 */

import (
	"context"

	"github.com/redis/go-redis/v9"
	extc "github.com/reugn/go-streams/extension"
	"github.com/reugn/go-streams/flow"
	extr "github.com/reugn/go-streams/redis"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
)

type Writer struct {
	source  *extc.ChanSource
	mapFlow *flow.PassThrough
	sink    *extr.PubSubSink
	in      chan interface{}
}

func NewWriter(config pipeline.WriterParams) io.Writer {
	options := &redis.Options{
		Addr:     *config.Host,    // use default Addr
		Password: config.Password, // no password set
		DB:       config.DB,       // use default DB
	}

	writer := Writer{
		mapFlow: flow.NewPassThrough(),
		sink:    extr.NewPubSubSink(context.Background(), redis.NewClient(options), config.Channel),
		in:      make(chan interface{}),
	}
	writer.source = extc.NewChanSource(writer.in)
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
