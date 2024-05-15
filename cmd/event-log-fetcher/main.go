package main

import (
	"sync"

	f "nuance.xaas-logging.event-log-collector/cmd/event-log-fetcher/fetcher"
	"nuance.xaas-logging.event-log-collector/cmd/utils"
)

// Print logo and run event log fetcher
func main() {
	utils.PrintLogo()

	var wg sync.WaitGroup
	wg.Add(1)
	f.Run(&wg)
	wg.Wait()
}
