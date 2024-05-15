package embedded

/*
 * embedded reader reads from a filesystem-backed persistent FIFO queue.
 */

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"heckel.io/pqueue"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
	p "nuance.xaas-logging.event-log-collector/pkg/pipeline"
	t "nuance.xaas-logging.event-log-collector/pkg/pipeline/io/types"
	"nuance.xaas-logging.event-log-collector/pkg/pipeline/io/utils"
)

type Reader struct {
	cfg p.ReaderParams
	pq  *pqueue.Queue
}

func NewReader(config p.ReaderParams) t.Reader {
	return &Reader{
		cfg: config,
	}
}

func (r *Reader) Open() (err error) {
	err = os.MkdirAll(*r.cfg.Path, os.ModePerm)
	if err != nil {
		return err
	}

	r.pq, err = pqueue.New(*r.cfg.Path)
	return err
}

func (r *Reader) Close() {
	// Nothing to do...
}

func (r *Reader) decompressData(compressed []byte) []byte {
	buf := bytes.NewBuffer(compressed)
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return compressed
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return compressed
	}
	return data
}

func (r *Reader) Read() ([]byte, error) {
	r.Open()

	data, err := r.pq.Dequeue()
	if err == nil {
		log.Debugf("read %d bytes of data from queue: %v", len(data), *r.cfg.Path)
		decompressed := r.decompressData(data)
		record, err := utils.PreProcessRedactedFields(decompressed)
		if err != nil {
			return decompressed, fmt.Errorf(t.InvalidPayloadError, err, string(decompressed))
		}
		return record, nil
	}
	return data, err
}
