package types

// TODO: this is a placeholder for providing clients a way to get a writer's status
type LogWriterStatus struct{}

type LogWriter interface {
	Start() error
	Stop()
	GetStatus() (LogWriterStatus, error)
}

// Factory method definition
type LogWriterFactory func(string) LogWriter

type Storage interface {
	Read(key string, path string) ([]byte, error)
	Write(key string, path string, data []byte) (int, error)
	Delete(key string, path string) error
	Shutdown()
}
