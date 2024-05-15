package workerpool

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type Handler func(interface{})

type WorkerPool struct {
	ID            string
	name          string
	size          int
	ch            chan interface{}
	signal        chan bool
	handler       Handler
	awaitShutdown *sync.WaitGroup

	started atomic.Bool
}

func NewWorkerPool(name string, ch chan interface{}, size int, handler Handler) *WorkerPool {
	id := uuid.New()
	return &WorkerPool{
		ID:            id.String(),
		name:          name,
		size:          size,
		ch:            ch,
		signal:        make(chan bool),
		handler:       handler,
		awaitShutdown: &sync.WaitGroup{},
		started:       atomic.Bool{},
	}
}

func (w *WorkerPool) runHandler(nbr int) {
	for {
		select {
		case data := <-w.ch:
			start := time.Now()
			id := uuid.New()
			log.Debugf("%v [#%v] worker [%v] calling handler: [%v]...", w.name, nbr, w.ID, id.String())
			w.handler(data)
			log.Debugf("%v [#%v] handler [%v] finished in %v ms [%v]...", w.name, nbr, w.ID, time.Since(start).Milliseconds(), id.String())
		case <-w.signal:
			log.Debugf("%v [#%v] received shutdown signal", w.name, nbr)
			w.awaitShutdown.Done()
			return
		}
	}
}

func (w *WorkerPool) Start() {
	if w.started.Load() {
		return
	}
	defer w.started.Store(true)

	for i := 0; i < w.size; i++ {
		w.awaitShutdown.Add(1)
		go w.runHandler(i)
	}
}

func (w *WorkerPool) Stop() {
	if !w.started.Load() {
		return
	}
	defer w.started.Store(false)

	for i := 0; i < w.size; i++ {
		w.signal <- true
	}
	w.awaitShutdown.Wait()
	log.Debugf("%v done shutting down", w.name)
}
