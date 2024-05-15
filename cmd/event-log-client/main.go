package main

import (
	"context"
	"sync"
	"syscall"

	f "nuance.xaas-logging.event-log-collector/cmd/event-log-fetcher/fetcher"
	p "nuance.xaas-logging.event-log-collector/cmd/event-log-processor/processor"
	w "nuance.xaas-logging.event-log-collector/cmd/event-log-writer/writer"
	"nuance.xaas-logging.event-log-collector/cmd/utils"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	sys "nuance.xaas-logging.event-log-collector/pkg/sys"
)

/*
 * event-log-client provides a stand-alone client that wraps each event log
 * component (fetcher, processor, writer) into a single executable, rather
 * than running each component separately.
 */
func main() {
	// Print the app logo and initialize logging
	utils.PrintLogo()
	log.Setup()

	// Setup signaling to capture program kill events and allow for graceful shutdown
	var wg sync.WaitGroup
	wg.Add(3)
	signal := sys.NewSignal(syscall.SIGINT, syscall.SIGTERM, syscall.SIGILL)

	// context with cancel for graceful shutdown
	_, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	// Launch each component
	go f.Run(&wg)
	go p.Run(&wg)
	go w.Run(&wg)

	// wait for signal to stop consuming/writing
	signal.ReceiveShutDown() // blocks until shutdown signal
	//log.Infof("\n*********************************\nGraceful shutdown signal received\n*********************************")
	log.Infof("GRACEFUL SHUTDOWN STARTED [ event-log-client ]....")
	cancelFunc() // Signal cancellation to context.Context
	wg.Wait()
	log.Infof("GRACEFUL SHUTDOWN COMPLETE [ event-log-client ].")
	//go signal.SendDone(true) // send shutdown done signal to receiver
	//log.Infof("\n*********************************\nGraceful shutdown completed      \n*********************************")
}
