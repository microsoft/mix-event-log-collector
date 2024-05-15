package neap

/*
 * Jobs represent actions to be taken by the storage writer. They are
 * channeled to worker pools for fan-out scaling
 */

const (
	DEFAULT_NUM_WORKERS = 1
)

type Action int

const (
	WRITE = iota
	READ
	DELETE
)

type Job struct {
	Index  string // Record index
	DocID  string // Record ID
	Body   []byte // Record conent
	Action Action // Action to take with storage provider
}

// NewJob() creates a new intance of a Job
func NewJob(action Action, key string, path string, data []byte) *Job {
	if key == "" || path == "" {
		return nil
	}
	if data == nil {
		return nil
	}
	return &Job{
		Index:  key,
		DocID:  path,
		Body:   data,
		Action: action,
	}
}
