package embedded

/*
 * embedded writer writes to a filesystem-backed persistent FIFO queue.
 * Records are compressed before being enqueued.
 */

import (
	"bytes"
	"compress/gzip"
	"os"

	"heckel.io/pqueue"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	p "nuance.xaas-logging.event-log-collector/pkg/pipeline"
	t "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
)

type Writer struct {
	cfg p.WriterParams
	pq  *pqueue.Queue
}

func NewWriter(config p.WriterParams) t.Writer {
	return &Writer{
		cfg: config,
	}
}

func (w *Writer) Open() (err error) {
	err = os.MkdirAll(*w.cfg.Path, os.ModePerm)
	if err != nil {
		return err
	}

	w.pq, err = pqueue.New(*w.cfg.Path)
	return err
}

func (w *Writer) Close() {
	// Nothing to do...
}

func (w *Writer) compressData(data []byte) []byte {
	var buf bytes.Buffer
	compr := gzip.NewWriter(&buf)
	compr.Write(data)
	compr.Close()
	return buf.Bytes()
}

func (w *Writer) Write(data []byte) error {
	compressed := w.compressData(data)
	log.Debugf("writing %d bytes of data to queue: %v", len(compressed), *w.cfg.Path)
	return w.pq.Enqueue(compressed)
}
