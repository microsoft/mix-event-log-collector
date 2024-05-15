package fluentd

/*
 * FluentClient implements StorageProvider supporting writing of event log
 * data to fluentd's in_forward TCP socket input plugin
 */

import (
	"context"
	"fmt"
	"time"

	fluent "github.com/lestrrat-go/fluent-client"
	"github.com/prometheus/client_golang/prometheus"
	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	"nuance.xaas-logging.event-log-collector/pkg/logging"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

type FluentClient struct {
	endpoint string
	client   fluent.Client
	done     chan bool
	pingErr  chan error
}

//// Public methods

// NewStorage() creates a new instance of a fluentd storage provider
func NewStorage(address string, buffered bool, bufferLimit int) wt.Storage {

	client, err := fluent.New(
		fluent.WithAddress(address),
		fluent.WithBuffered(buffered),
		fluent.WithBufferLimit(bufferLimit))
	if err != nil {
		// fluent.New may return an error if invalid values were
		// passed to the constructor
		logging.Fatalf("failed to create fluentd client: %s", err)
		return nil
	}

	s := &FluentClient{
		endpoint: address,
		client:   client,
		done:     make(chan bool),
		pingErr:  make(chan error),
	}

	// monitor the fluentd endpoint
	s.monitor()

	return s
}

// monitor() pings the fluentd endpoint once every minute to check for liveness
func (f *FluentClient) monitor() {
	go func() {
		for {
			select {
			case <-f.done:
				logging.Infof("stopping fluentd endpoint monitor")
				return
			case e := <-f.pingErr:
				logging.Errorf("%s", e.Error())
			}
		}
	}()

	go fluent.Ping(context.Background(), f.client, "ping", "event-log-writer monitor", fluent.WithPingInterval(60*time.Second), fluent.WithPingResultChan(f.pingErr))
}

//// Interface methods

// Read() reads an event log from fluentd. Not implemented.
func (f *FluentClient) Read(key string, path string) ([]byte, error) {
	return nil, fmt.Errorf("not Implemented")
}

// Write() writes an event log to fluentd
func (f *FluentClient) Write(key string, path string, data []byte) (int, error) {
	start := time.Now()
	var err error

	// Observe write duration in seoncds and set histogram and guage metric
	timer := prometheus.NewTimer(CommitDurationHistogram)

	defer func() {
		// monitor how long it takes to commit records to mongodb
		CommitDurationHistogram.Observe(timer.ObserveDuration().Seconds())

		// monitor processing duration
		monitoring.SetGauge(wt.WriteDurationGauge,
			time.Since(start).Seconds(),
			key)

		if err != nil {
			logging.Errorf("failed to write record: %v", err)
			monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_FLUENTD, wt.NOT_WRITTEN, err.Error())
			monitoring.IncCounter(CommitTotalCounter, fmt.Sprintf("%v:%v", f.endpoint, key), err.Error())
		} else {
			logging.Infof("successfully wrote record [id = %v]", path)
			monitoring.IncCounter(wt.WritesTotalCounter, wt.LABEL_FLUENTD, wt.WRITTEN, "")
			monitoring.IncCounter(CommitTotalCounter, fmt.Sprintf("%v:%v", f.endpoint, key), monitoring.PROM_STATUS_SUCCESS)
		}
	}()

	// Write event log to fluentd TCP endpoint
	err = f.client.Post(key, string(data), fluent.WithSyncAppend(true))
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

// Delete() deletes an event log from fluentd. Not implemented.
func (f *FluentClient) Delete(key string, path string) error {
	return nil
}

// Shutdown() signals the fluentd storage provider to shutdown
func (f *FluentClient) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f.done <- true
	if err := f.client.Shutdown(ctx); err != nil {
		logging.Errorf("failed to shutdown fluentd client properly. force-closing it")
		f.client.Close()
	}

	logging.Debugf("fluentd client shutdown complete")
}
