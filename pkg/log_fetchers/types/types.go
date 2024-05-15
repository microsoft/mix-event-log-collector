package types

// TODO: this is a placeholder for providing clients a way to get a fetcher's status
type LogFetcherStatus struct{}

type LogFetcher interface {
	Start() error
	Stop()
	GetStatus() (LogFetcherStatus, error)
}

// Factory method definition
type LogFetcherFactory func(string) LogFetcher

// Type definitions based on log.api kafka response objects

type ConsumerGroup string
type Consumers []Consumer

type Consumer struct {
	Group     ConsumerGroup
	Name      string
	Partition int
}

type Subscription struct {
	Consumer Consumer
}

type Partition struct {
	Topic     string `json:"topic,omitempty"`
	Partition string `json:"partition,omitempty"`
}

type Partitions struct {
	Partitions []Partition `json:"partitions,omitempty"`
}

type PartitionsResponse struct {
	Topic      string  `json:"topic"`
	Partitions []int64 `json:"partitions"`
}

type Offset struct {
	Topic     string `json:"topic"`
	Partition int64  `json:"partition"`
	Offset    int64  `json:"offset"`
}

type Offsets struct {
	Offsets []Offset `json:"offsets"`
}

type OffsetRangeResponse struct {
	BeginningOffset int64 `json:"beginning_offset"`
	EndOffset       int64 `json:"end_offset"`
}

type OffsetDetails struct {
	Topic           string `json:"topic"`
	Partition       int64  `json:"partition"`
	CurrentOffset   int64  `json:"current_offset"`
	BeginningOffset int64  `json:"beginning_offset"`
	EndOffset       int64  `json:"end_offset"`
}
