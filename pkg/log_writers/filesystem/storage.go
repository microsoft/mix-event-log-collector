package filesystem

/*
 * Filesystem implements StorageProvider supporting writing of event log
 * data to the local filesystem
 */

import (
	"fmt"
	"os"
	"time"

	wt "nuance.xaas-logging.event-log-collector/pkg/log_writers/types"
	"nuance.xaas-logging.event-log-collector/pkg/monitoring"
)

type Filesystem struct {
	RootDir string // top-level dir to write event logs to
}

// File represents a file to be written to disk
type File struct {
	subDir string // file path (excludes root dir)
	name   string // file name
	data   []byte // file content
}

//// Public methods

// NewFile() creates a new instance of a File object
func NewFile(subDir string, name string, data []byte) File {
	return File{
		subDir: subDir,
		name:   name,
		data:   data}
}

// NewStorage() creates a new instance of a filesystem storage provider
func NewStorage(path string) wt.Storage {

	fs := &Filesystem{
		RootDir: path,
	}
	return fs
}

//// Interface methods

// Read() reads a file from the filesystem. Not implemented.
func (f *Filesystem) Read(key string, path string) ([]byte, error) {
	return nil, fmt.Errorf("not Implemented")
}

// Write() writes a file to the filesystem
func (f *Filesystem) Write(key string, path string, data []byte) (int, error) {
	start := time.Now()
	var err error

	defer func() {
		// monitor processing duration
		monitoring.SetGauge(wt.WriteDurationGauge,
			time.Since(start).Seconds(),
			key)

		if err != nil {
			monitoring.IncCounter(wt.WritesTotalCounter,
				"filesystem",
				wt.NOT_WRITTEN,
				err.Error())
		} else {
			monitoring.IncCounter(wt.WritesTotalCounter,
				"filesystem",
				wt.WRITTEN,
				"")
		}
	}()

	// Ensure the path we want to write to exists
	err = os.MkdirAll(fmt.Sprintf("%s/%s", f.RootDir, key), 0750)
	if err != nil {
		return 0, err
	}

	// Write the event log file to the specified path and name
	err = os.WriteFile(fmt.Sprintf("%s/%s/%s", f.RootDir, key, path), data, 0600)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

// Delete() deletes a file from the filesystem. Not implemented.
func (f *Filesystem) Delete(key string, path string) error {
	err := os.Remove(fmt.Sprintf("%s/%s", key, path))
	if err != nil {
		return err
	}
	return nil
}

// Shutdown() signals the filesystem storage provider to shutdown
func (f *Filesystem) Shutdown() {
	// Nothing to do...
}
