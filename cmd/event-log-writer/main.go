package main

import (
	"sync"

	w "nuance.xaas-logging.event-log-collector/cmd/event-log-writer/writer"
	"nuance.xaas-logging.event-log-collector/cmd/utils"
)

// Print logo and run event log writer
func main() {
	utils.PrintLogo()

	var wg sync.WaitGroup
	wg.Add(1)
	w.Run(&wg)
	wg.Wait()
}
