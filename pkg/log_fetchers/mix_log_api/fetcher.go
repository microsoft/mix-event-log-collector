package mix_log_api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v2"

	"nuance.xaas-logging.event-log-collector/pkg/httpclient"
	ct "nuance.xaas-logging.event-log-collector/pkg/log_fetchers/types"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
	oauth "nuance.xaas-logging.event-log-collector/pkg/oauth"
	pipeline "nuance.xaas-logging.event-log-collector/pkg/pipeline"
	io "nuance.xaas-logging.event-log-collector/pkg/pipeline/io"
	iot "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	t "nuance.xaas-logging.event-log-collector/pkg/types"
)

const (
	// Kafka rest api paths
	ConsumerPath          = "/consumers"
	SubscriptionPath      = "/consumers/subscription"
	RecordsPath           = "/consumers/records"
	AssignmentsPath       = "/consumers/assignments"
	PartitionsPath        = "/partitions"
	PartitionsOffsetsPath = "/partitions/%d/offsets"
	ConsumersOffsetsPath  = "/consumers/offsets"
	CommittedOffsetsPath  = "/consumers/committed/offsets"
	PositionsPath         = "/consumers/positions"
	FirstOffsetPath       = "/consumers/positions/beginning"
	LastOffsetPath        = "/consumers/positions/end"
)

var (
	DEFAULT_CONNECT_TIMEOUT_MS = 60000

	// Default mix-log-api fetcher config
	DefaultLogFetcherYaml = LogFetcherYaml{
		Fetcher: LogFetcherParams{
			Name: "mix-log-api",
			Config: Config{
				Credentials: Credentials{
					AuthDisabled:  false,
					TokenURL:      "https://auth.crt.nuance.com/oauth2/token",
					Scope:         "log",
					AuthTimeoutMS: 5000,
				},
				ApiUrl:               "https://log.api.nuance.com",
				ConnectTimeoutMs:     DEFAULT_CONNECT_TIMEOUT_MS,
				MaxRetries:           -1,
				Delay:                5,
				RecordCheckFrequency: 10,
				ClientName:           "neap",
				ConsumerGroupID:      "01",
				ConsumerOptions:      t.JsonObj{"auto.offset.reset": "latest"},
			},
		},
	}
)

// The following structs represent a log fetcher yaml configuration structure
type LogFetcherYaml struct {
	Fetcher LogFetcherParams `json:"fetcher" yaml:"fetcher"`
}

type LogFetcherParams struct {
	Name   string `json:"name" yaml:"name"`
	Config Config `json:"config" yaml:"config"`
}

type Credentials struct {
	AuthDisabled  bool   `json:"auth_disabled" yaml:"auth_disabled"`
	TokenURL      string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	ClientID      string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	ClientSecret  string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
	Scope         string `json:"scope,omitempty" yaml:"scope,omitempty"`
	AuthTimeoutMS int    `json:"auth_timeout_ms,omitempty" yaml:"auth_timeout_ms,omitempty"`
}

type Config struct {
	Credentials          Credentials `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	ApiUrl               string      `json:"api_url,omitempty" yaml:"api_url,omitempty"`
	ConnectTimeoutMs     int         `json:"connect_timeout_ms,omitempty" yaml:"connect_timeout_ms,omitempty"`
	RateLimit            *int        `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty"`
	MaxRetries           int         `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
	Delay                int         `json:"retry_delay,omitempty" yaml:"retry_delay,omitempty"`
	RecordCheckFrequency int         `json:"record_check_frequency,omitempty" yaml:"record_check_frequency,omitempty"`
	ClientName           string      `json:"client_name" yaml:"client_name"`
	ConsumerGroupID      string      `json:"consumer_group_id,omitempty" yaml:"consumer_group_id,omitempty"`
	ConsumerOptions      t.JsonObj   `json:"consumer_options,omitempty" yaml:"consumer_options,omitempty"`
	Partitions           []float64   `json:"partitions,omitempty" yaml:"partitions,omitempty"`
}

// LogFetcher is the mix-log-api LogFetcher implementation
type LogFetcher struct {
	httpclient.Http // mix log.api http client

	name          string               // the log fetcher name
	cfg           Config               // the log fetcher configuration
	Authenticator *oauth.Authenticator // oauth2 authentication handler

	// The following members represent log.api kafka objects
	groupName    ct.ConsumerGroup
	consumers    ct.Consumers
	subscription *ct.Subscription

	topic      string
	clientName string

	// offset settings
	offsetReset       string
	autoCommitEnabled bool

	Partitions        []int64             // the list of kafka partitions that belong to a consumer
	partitionFetchers []*PartitionFetcher // the list of partition fetchers which will fetch and process event logs for each partition

	shutdown chan bool      // shutdown signal
	wait     sync.WaitGroup // wait for partition fetchers to complete fetching/processing of records

	writer iot.Writer // data pipeline writer
}

//// Public methods

// NewLogFetcher() creates a new instance of a mix-log-api LogFetcher
func NewLogFetcher(configFile string) ct.LogFetcher {
	log.Debugf("factory creating new mix-log-api fetcher")

	cfg, err := loadConfigFromFile(configFile)
	if err != nil {
		log.Fatalf("%v", err)
	}

	auth := createAuthenticator(*cfg)

	timeout := cfg.Fetcher.Config.ConnectTimeoutMs
	if timeout <= 0 {
		timeout = DEFAULT_CONNECT_TIMEOUT_MS
	}

	offsetReset := ""
	if autoReset, ok := cfg.Fetcher.Config.ConsumerOptions["auto.offset.reset"]; ok {
		offsetReset = autoReset.(string)
	}

	autoCommitEnabled := false
	if autoCommit, ok := cfg.Fetcher.Config.ConsumerOptions["auto.commit.enable"]; ok {
		autoCommitEnabled, _ = autoCommit.(bool)
	}

	fetcher := LogFetcher{
		name:          cfg.Fetcher.Name,
		cfg:           cfg.Fetcher.Config,
		Authenticator: auth,
		Http: httpclient.Http{
			HttpClient: http.Client{
				Timeout: time.Duration(time.Duration(timeout) * time.Millisecond),
			},
		},
		offsetReset:       offsetReset,
		autoCommitEnabled: autoCommitEnabled,
	}
	fetcher.writer, _ = io.CreateWriter(pipeline.FETCHER, configFile)

	// Parse clientID into it's components and initialize consumer topics, clientName, and group
	parts := fetcher.parseClientID(cfg.Fetcher.Config.Credentials.ClientID)
	appID := parts["appID"]
	fetcher.topic = appID

	// If client id contains an AppID k/v pair, use that for client name. Otherwise, use the one provided from config file
	// which provides backward compatibility support for original client id naming conventions which did not include
	// k/v pairs in the client_id (client name, geo, etf.)
	if _, ok := parts["clientName"]; ok {
		fetcher.clientName = parts["clientName"]
	} else {
		fetcher.clientName = fetcher.cfg.ClientName
	}

	fetcher.groupName = ct.ConsumerGroup(fmt.Sprintf("appID-%v-clientName-%v-%v", appID, fetcher.clientName, fetcher.cfg.ConsumerGroupID))

	return &fetcher
}

//// Helper methods

// loadConfigFromFile() loads the fetcher yaml config from file. Over-rides config file settings with env vars if set.
func loadConfigFromFile(configFile string) (*LogFetcherYaml, error) {
	// Start with defaults
	yml := DefaultLogFetcherYaml

	// Read config file content
	file, err := os.ReadFile(filepath.Clean(configFile))
	if err == nil {
		// Unmarshall yaml
		err = yaml.Unmarshal(file, &yml)
		if err != nil {
			return nil, err
		}
	}

	// Over-ride some fetcher config values with env vars if they've been set
	yml.Fetcher.Config.Credentials.ClientID = getValueFromEnv("ELC_CLIENT_ID", yml.Fetcher.Config.Credentials.ClientID)
	yml.Fetcher.Config.Credentials.ClientSecret = getValueFromEnv("ELC_CLIENT_SECRET", yml.Fetcher.Config.Credentials.ClientSecret)
	yml.Fetcher.Config.Credentials.TokenURL = getValueFromEnv("ELC_TOKEN_URL", yml.Fetcher.Config.Credentials.TokenURL)
	yml.Fetcher.Config.ApiUrl = getValueFromEnv("ELC_API_URL", yml.Fetcher.Config.ApiUrl)
	yml.Fetcher.Config.ConsumerGroupID = getValueFromEnv("ELC_CONSUMER_GROUP_ID", yml.Fetcher.Config.ConsumerGroupID)

	raw, _ := json.Marshal(log.MaskSensitiveData(yml))
	log.Debugf("%v", string(raw))

	return &yml, nil
}

// createAuthenticator() returns an instance of oauth.Authenticator
func createAuthenticator(cfg LogFetcherYaml) *oauth.Authenticator {
	auth := oauth.NewAuthenticator(oauth.Credentials(cfg.Fetcher.Config.Credentials))
	return auth
}

// getValueFromEnv() returns the value of the env var if available. If not available, returns the default value
func getValueFromEnv(name string, defaultVal string) string {
	if val, found := os.LookupEnv(name); found {
		return val
	}
	return defaultVal
}

//// Private methods

// logOffsetDetails() is a helper function for logging and monitoring Offset details
func (f *LogFetcher) logOffsetDetails(offset ct.OffsetDetails) {
	log.Infof("Partition [%v] offset details: {\"start\": %d, \"end\": %d, \"current\": %d, \"total\": %d, \"behind\": %d}",
		offset.Partition,
		offset.BeginningOffset,
		offset.EndOffset,
		offset.CurrentOffset,
		offset.EndOffset-offset.BeginningOffset,
		offset.EndOffset-offset.CurrentOffset)

	monitoring.SetGauge(ct.ConsumerOffsetLowGauge, float64(offset.BeginningOffset), offset.Topic, fmt.Sprint(offset.Partition))
	monitoring.SetGauge(ct.ConsumerOffsetHighGauge, float64(offset.EndOffset), offset.Topic, fmt.Sprint(offset.Partition))
	monitoring.SetGauge(ct.ConsumerOffsetCurrentGauge, float64(offset.CurrentOffset), offset.Topic, fmt.Sprint(offset.Partition))
	if offset.EndOffset-offset.CurrentOffset >= 0 {
		monitoring.SetGauge(ct.ConsumerOffsetBehindGauge, float64(offset.EndOffset-offset.CurrentOffset), offset.Topic, fmt.Sprint(offset.Partition))
	}
}

// parseClientID() parses the ClientID into an array of key/value pairs. The client id uses a colon
// to separate it's parts. e.g. appID:<value>:geo:<value>:clientName:<value>
func (f *LogFetcher) parseClientID(clientID string) map[string]string {

	arr := strings.Split(clientID, ":")

	kvs := make(map[string]string)

	for i := 0; i < len(arr)-1; i += 2 {
		kvs[arr[i]] = arr[i+1]
	}
	return kvs
}

// header() creates a default HTTP header with bearer token and consumer group and name
func (f *LogFetcher) header(token oauth.Token, name string) map[string][]string {
	if len(token.AccessToken) == 0 {
		return map[string][]string{
			"Content-Type":       {"application/json"},
			"consumer-group":     {string(f.groupName)},
			"consumer-name":      {name},
			"x-nuance-client-id": {f.cfg.Credentials.ClientID},
		}
	}

	return map[string][]string{
		"Authorization":  {fmt.Sprintf("%s %s", token.TokenType, token.AccessToken)},
		"Content-Type":   {"application/json"},
		"consumer-group": {string(f.groupName)},
		"consumer-name":  {name},
	}
}

// parseURL() appends the api path to the consumer's configured base url and returns the
// full path as a parsed URL object
func (f *LogFetcher) parseURL(apiPath string) *url.URL {
	url, _ := url.Parse(fmt.Sprintf("%s%s", f.cfg.ApiUrl, apiPath))
	return url
}

//// Kafka consumer and partition lifecycle methods...

// createConsumer() creates an instance of a kafka consumer
func (f *LogFetcher) createConsumer() (*ct.Consumer, error) {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return nil, err
	}

	data, err := json.Marshal(f.cfg.ConsumerOptions)
	if err != nil {
		return nil, err
	}

	url := f.parseURL(ConsumerPath)

	// To ensure uniqueness on the server-side, use a uuid in the consumer name
	// Note that consumer instances are deleted server-side if idle for 2 min
	uid, _ := uuid.NewUUID()
	name := fmt.Sprintf("consumer-%s", uid.String())
	consumer := ct.Consumer{
		Group:     f.groupName,
		Name:      name,
		Partition: -1,
	}
	f.consumers = append(f.consumers, consumer)

	statusCode, response, err := f.Http.Post(url, f.header(*token, name), data)
	if err != nil {
		return nil, err
	}

	switch {
	case statusCode == 204:
		return &consumer, nil
	default:
		return nil, errors.New(string(response))
	}
}

// createSubscription() creates a consumer subscription
func (f *LogFetcher) createSubscription(consumer ct.Consumer) (*ct.Subscription, error) {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return nil, err
	}

	data, err := json.Marshal(map[string][]string{
		"topics": {f.topic},
	})
	if err != nil {
		return nil, err
	}

	url := f.parseURL(SubscriptionPath)
	statusCode, response, err := f.Http.Post(url, f.header(*token, consumer.Name), data)
	if err != nil {
		return nil, err
	}

	switch {
	case statusCode == 204:
		return &ct.Subscription{Consumer: consumer}, nil
	default:
		return nil, errors.New(string(response))
	}
}

// getPartitions() gets the list of available partitions for a consumer
func (f *LogFetcher) getPartitions(consumer ct.Consumer) ([]int64, error) {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return nil, err
	}

	url := f.parseURL(PartitionsPath)

	code, body, err := f.Http.Get(url, f.header(*token, consumer.Name))
	if err != nil {
		return nil, err
	}
	switch {
	case code == 200:
		var partitions ct.PartitionsResponse
		err = json.Unmarshal(body, &partitions)
		return partitions.Partitions, err
	default:
		return nil, errors.New(string(body))
	}
}

// assignPartitionToConsumer() assigns a partition to a consumer
func (f *LogFetcher) assignPartitionToConsumer(partition int, consumer ct.Consumer) (*ct.Consumer, error) {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return nil, err
	}

	url := f.parseURL(AssignmentsPath)

	p := ct.Partitions{}
	p.Partitions = append(p.Partitions, ct.Partition{Topic: f.topic, Partition: fmt.Sprintf("%d", partition)})

	data, _ := json.Marshal(p)
	code, response, err := f.Http.Post(url, f.header(*token, consumer.Name), data)
	switch {
	case err != nil:
		return nil, err
	case code == 204:
		// Update the previously cached consumer
		for idx, entry := range f.consumers {
			if entry.Name == consumer.Name {
				entry.Partition = partition
				f.consumers[idx] = entry
				return &entry, nil
			}
		}
		return nil, fmt.Errorf("consumer %s not found in cache", consumer.Name)
	default:
		return nil, errors.New(string(response))
	}
}

// getOffsetRange() gets the beginning and end offset range of a partition
func (f *LogFetcher) getOffsetRange(consumer ct.Consumer, partition int) (*ct.OffsetRangeResponse, error) {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return nil, err
	}

	url := f.parseURL(fmt.Sprintf(PartitionsOffsetsPath, partition))

	code, body, err := f.Http.Get(url, f.header(*token, consumer.Name))
	switch {
	case err != nil:
		return nil, err
	case code >= 300:
		return nil, fmt.Errorf("%v: %v", code, string(body))
	default:
		// E.g. {"beginning_offset":7817,"end_offset":13222}
		var offsetRange ct.OffsetRangeResponse
		err = json.Unmarshal(body, &offsetRange)
		return &offsetRange, err
	}
}

// getCurrentOffset() gets the current offset of a given partition
func (f *LogFetcher) getCurrentOffset(consumer ct.Consumer, partition int) (*ct.Offset, error) {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return nil, err
	}

	url := f.parseURL(CommittedOffsetsPath)

	p := ct.Partitions{}
	p.Partitions = append(p.Partitions, ct.Partition{Topic: f.topic, Partition: fmt.Sprintf("%d", consumer.Partition)})

	data, _ := json.Marshal(p)
	code, body, err := f.Http.Post(url, f.header(*token, consumer.Name), data)
	switch {
	case err != nil:
		return nil, err
	case code >= 300:
		return nil, fmt.Errorf("%v: %v", code, string(body))
	default:
		var offsets ct.Offsets
		err = json.Unmarshal(body, &offsets)
		if len(offsets.Offsets) == 0 {
			offsets.Offsets = append(offsets.Offsets, ct.Offset{
				Topic:     f.topic,
				Partition: 0,
				Offset:    0,
			})
		}
		return &offsets.Offsets[0], err
	}
}

// getOffsetDetails() gets the complete offset details of a consumer's partition
func (f *LogFetcher) getOffsetDetails(consumer ct.Consumer) (*ct.OffsetDetails, error) {
	offsetRange, err := f.getOffsetRange(consumer, consumer.Partition)
	if err != nil {
		return nil, err
	}

	currentOffset, err := f.getCurrentOffset(consumer, consumer.Partition)
	if err != nil {
		return nil, err
	}

	var offsetDetails ct.OffsetDetails
	offsetDetails.Partition = currentOffset.Partition
	offsetDetails.Topic = currentOffset.Topic
	offsetDetails.BeginningOffset = offsetRange.BeginningOffset
	offsetDetails.EndOffset = offsetRange.EndOffset

	// Mix log.api can return an offset value that is less than the beginning offset (records have never been fetched for the given partition)
	// as well as greater than the latest offset (all records have been fetched and no new ones have been generated). This causes problems
	// when trying to calculate how far behind we are with fetching data from the partition. So here we simply align current offset to always be
	// within the bounds of beginning and end offsets
	switch {
	case currentOffset.Offset < offsetDetails.BeginningOffset:
		offsetDetails.CurrentOffset = offsetDetails.BeginningOffset
	case currentOffset.Offset > offsetDetails.EndOffset:
		offsetDetails.CurrentOffset = offsetDetails.EndOffset
	default:
		offsetDetails.CurrentOffset = currentOffset.Offset
	}

	return &offsetDetails, nil
}

// seekFirstOffset() sets the cursor position to the earliest available offset of a consumer's partition
func (f *LogFetcher) seekFirstOffset(consumer ct.Consumer) error {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return err
	}

	url := f.parseURL(FirstOffsetPath)

	p := ct.Partitions{}
	p.Partitions = append(p.Partitions, ct.Partition{Topic: f.topic, Partition: fmt.Sprintf("%d", consumer.Partition)})

	data, _ := json.Marshal(p)
	code, body, err := f.Http.Post(url, f.header(*token, consumer.Name), data)
	switch {
	case err != nil:
		return err
	case code == 204:
		log.Debugf("offset reset to first position (%v)", consumer)
		return nil
	default:
		return errors.New(string(body))
	}
}

// seekLastOffset() sets the cursor position to the latest available offset of a consumer's partition
func (f *LogFetcher) seekLastOffset(consumer ct.Consumer) error {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return err
	}

	url := f.parseURL(LastOffsetPath)

	p := ct.Partitions{}
	p.Partitions = append(p.Partitions, ct.Partition{Topic: f.topic, Partition: fmt.Sprintf("%d", consumer.Partition)})

	data, _ := json.Marshal(p)
	code, body, err := f.Http.Post(url, f.header(*token, consumer.Name), data)
	switch {
	case err != nil:
		return err
	case code == 204:
		log.Debugf("offset reset to last position (%v)", consumer)
		return nil
	default:
		return errors.New(string(body))
	}
}

// commitOffset saves the cursor position of the consumer's partition
func (f *LogFetcher) commitOffset(consumer ct.Consumer, offset int) error {
	token, err := f.Authenticator.Authenticate()
	if err != nil {
		log.Errorf("%s", err)
		return err
	}

	url := f.parseURL(ConsumersOffsetsPath)

	offsets := ct.Offsets{}
	offsets.Offsets = append(offsets.Offsets, ct.Offset{
		Topic:     f.topic,
		Partition: int64(consumer.Partition),
		Offset:    int64(offset),
	})

	data, err := json.Marshal(&offsets)
	if err != nil {
		return err
	}

	code, body, err := f.Http.Post(url, f.header(*token, consumer.Name), data)
	switch {
	case err != nil:
		return err
	case code < 300:
		log.Debugf("offset committed to %v for partition %v", offset, consumer.Partition)
		return nil
	default:
		return errors.New(string(body))
	}
}

//// Interface methods

// Start() implements the LogFetcher interface. It starts async fetching of event log data
// across all available kafka partitions and writes each record to the data pipeline for
// downstream processing
func (f *LogFetcher) Start() error {
	log.Infof("starting %v log fetcher", f.name)

	// Create Kafka Consumer
	consumer, err := f.createConsumer()
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Debugf("%+v", consumer)

	// Create Kafka Subscription
	f.subscription, err = f.createSubscription(*consumer)
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Debugf("%+v", f.subscription)

	// Get Partitions
	f.Partitions, err = f.getPartitions(*consumer)
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Debugf("%v", f.Partitions)
	monitoring.SetGauge(ct.PartitionTotalGauge, float64(len(f.Partitions)),
		f.name,
		f.topic,
		string(f.groupName),
		consumer.Name)

	// initialize shutdown signal channel, and open pipeline writer
	f.shutdown = make(chan bool)
	f.writer.Open()

	// for each partition
	// 	create and assign consumer
	// 	set each consumer's cursor position to desired offset
	// 	fetch and process data stream async
	f.partitionFetchers = make([]*PartitionFetcher, len(f.Partitions))
	for p := range f.Partitions {
		f.wait.Add(1)
		f.partitionFetchers[p], err = f.createPartitionFetcher(p)
		if err != nil {
			log.Fatal("%v", err)
		}
		go f.partitionFetchers[p].FetchRecords(&f.wait)

	}

	return nil
}

func (f *LogFetcher) ResetPartitionConsumer(partition int) (*ct.Consumer, error) {
	// create new consumer
	consumer, err := f.createConsumer()
	if err != nil {
		return nil, err
	}
	// assign partition to consumer
	consumer, err = f.assignPartitionToConsumer(partition, *consumer)
	if err != nil {
		return nil, err
	}
	return consumer, nil
}

func (f *LogFetcher) createPartitionFetcher(partition int) (*PartitionFetcher, error) {
	// create new consumer
	consumer, err := f.ResetPartitionConsumer(partition)
	if err != nil {
		return nil, err
	}

	// set cursor position if necessary
	switch f.offsetReset {
	case "earliest":
		// get offset details
		offsetDetails, err := f.getOffsetDetails(*consumer)
		if err != nil {
			return nil, err
		}
		f.commitOffset(*consumer, int(offsetDetails.BeginningOffset))
		f.seekFirstOffset(*consumer)

	case "latest":
		// get offset details
		offsetDetails, err := f.getOffsetDetails(*consumer)
		if err != nil {
			return nil, err
		}
		f.commitOffset(*consumer, int(offsetDetails.EndOffset))
		f.seekLastOffset(*consumer)
	default:
		// do nothing
	}

	// get offset details
	offsetDetails, err := f.getOffsetDetails(*consumer)
	if err != nil {
		log.Fatalf("%v", err)
	}
	f.logOffsetDetails(*offsetDetails)

	return NewPartitionFetcher(f, consumer, f.shutdown), nil
}

// Stop() implements the LogFetcher interface. It shuts down event log fetching and processing across all partitions
func (f *LogFetcher) Stop() {
	log.Debugf("stopping %v log fetcher", f.name)

	for idx := range f.partitionFetchers {
		log.Debugf("shutting down partition %v", idx)
		go func() {
			f.shutdown <- true // notify partition fetcher to shutdown
		}()
	}
	log.Debugf("waiting for partition fetchers to gracefully shutdown")
	f.wait.Wait()
	log.Debugf("partition fetchers have gracefully shutdown")
}

// GetStatus() implements the LogFetcher interface. Currently not doing much...
func (f *LogFetcher) GetStatus() (ct.LogFetcherStatus, error) {
	log.Debugf("getting %v log fetcher status", f.name)
	return ct.LogFetcherStatus{}, nil
}
