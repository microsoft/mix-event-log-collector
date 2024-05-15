package sys

import (
	"os"
	"os/signal"

	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type Signal struct {
	shutdown chan os.Signal
	done     chan bool
}

func NewSignal(signals ...os.Signal) *Signal {
	s := &Signal{
		shutdown: make(chan os.Signal),
		done:     make(chan bool),
	}
	s.Notify(signals...)
	return s
}
func (s *Signal) Notify(signals ...os.Signal) {
	signal.Notify(s.shutdown, signals...)
}

func (s *Signal) SendShutDown(signal os.Signal) {
	log.Debugf("sending shutdown signal:%s", signal)
	s.shutdown <- signal
}

func (s *Signal) ReceiveShutDown() {
	signal := <-s.shutdown
	log.Debugf("receiving shutdown signal:%s", signal)
}

func (s *Signal) SendDone(signal bool) {
	s.done <- signal
	log.Debugf("sent done signal: %v", signal)
}

func (s *Signal) ReceiveDone() {
	signal := <-s.done
	log.Debugf("receiving done signal: %v", signal)
}
