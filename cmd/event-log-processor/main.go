package main

import (
	"sync"

	p "nuance.xaas-logging.event-log-collector/cmd/event-log-processor/processor"
	"nuance.xaas-logging.event-log-collector/cmd/utils"
)

// Print logo and run event log processor
func main() {
	utils.PrintLogo()

	var wg sync.WaitGroup
	wg.Add(1)
	p.Run(&wg)
	wg.Wait()
}
