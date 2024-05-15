package kafka

/*
 * kafka reader reads from a kafka topic.
 */

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	extc "github.com/reugn/go-streams/extension"
	extk "github.com/reugn/go-streams/kafka"

	"github.com/IBM/sarama"
	"github.com/reugn/go-streams/flow"
	"nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/utils"
	"nuance.xaas-logging.event-log-collector/pkg/types"
)

type Reader struct {
	config  *sarama.Config
	source  *extk.KafkaSource
	sink    *extc.ChanSink
	mapFlow *flow.PassThrough
	out     chan interface{}
	ctx     context.Context
}

func NewReader(config pipeline.ReaderParams) io.Reader {
	ctx := context.Background()

	reader := Reader{
		mapFlow: flow.NewPassThrough(),
		out:     make(chan interface{}),
		ctx:     ctx,
	}
	reader.sink = extc.NewChanSink(reader.out)

	reader.config = sarama.NewConfig()
	reader.config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	reader.config.Version, _ = sarama.ParseKafkaVersion("2.8.1")
	reader.config.Consumer.Offsets.AutoCommit.Enable = config.AutoCommitOffsetEnabled

	switch config.Offset {
	case "latest":
		reader.config.Consumer.Offsets.Initial = sarama.OffsetNewest
	case "earliest":
		reader.config.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		// none
	}

	var groupID string
	// Create a unique group id to allow for multiple readers to consume from the same topic
	if config.CreateUniqueGroupID {
		uid, _ := uuid.NewUUID()
		groupID = fmt.Sprintf("%v-%v", *config.Group, uid.String())
		logging.Infof("kafka pipeline reader group id: %v", groupID)
	} else {
		groupID = *config.Group
	}

	var err error
	reader.source, err = extk.NewKafkaSource(reader.ctx,
		config.Hosts,
		groupID,
		reader.config,
		*config.Topic)

	if err != nil {
		logging.Errorf("%v", err)
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
	logging.Debugf("kafka reader closed")
}

// mapOnPremiseRecord checks to see if the structure of the record schema contained
// in the consumed message Value matches what is expected when fetched directly from
// Mix Hosted log.api. If not then we are dealing with an on-premise deployment of XaaS
// engines and we need to map the consumed message keys to match that of mix log.api
func (w *Reader) mapOnPremiseRecord(msg *sarama.ConsumerMessage) ([]byte, error) {

	// Pre-process the record to ensure redacted fields match expected data format (e.g. string vs json array)
	record, err := utils.PreProcessRedactedFields(msg.Value)
	if err != nil {
		return nil, fmt.Errorf(io.InvalidPayloadError, err, string(msg.Value))
	}

	var schema types.EventLogSchema
	err = json.Unmarshal(record, &schema)
	if err != nil {
		return nil, fmt.Errorf(io.InvalidPayloadError, err, string(msg.Value))
	}

	// If schema key fields are missing we need to map the record
	if schema.Key.ID == "" {
		var key types.EventLogKey
		var value types.EventLogValue

		// Parse kafka event log key
		err = json.Unmarshal(msg.Key, &key)
		if err != nil {
			return nil, fmt.Errorf(io.InvalidPayloadError, err, string(msg.Value))
		}

		// parse kafka event log value
		err = json.Unmarshal(msg.Value, &value)
		if err != nil {
			return nil, fmt.Errorf(io.InvalidPayloadError, err, string(msg.Value))
		}

		// build the schema...
		schema.Offset = float64(msg.Offset)
		schema.Partition = float64(msg.Partition)
		schema.Topic = msg.Topic
		schema.Key = key
		if len(strings.TrimSpace(schema.Key.ID)) == 0 {
			schema.Key.ID = value.ID
		}
		schema.Value = value

		// return the schema as bytes
		bytes, _ := json.Marshal(schema)
		return bytes, nil
	}
	return msg.Value, nil
}

func (w *Reader) Read() ([]byte, error) {
	data := <-w.out
	logging.Debugf("gostreamer read()  --> %+v", data)
	payload := data
	switch payload.(type) {
	case nil:
		return nil, errors.New("reader closed")
	case *sarama.ConsumerMessage:
		logging.Debugf("gostreamer read() payload  --> %+v", string(payload.(*sarama.ConsumerMessage).Value))
		return w.mapOnPremiseRecord(payload.(*sarama.ConsumerMessage))
	default:
		return nil, errors.New("unknown payload type")
	}
}
