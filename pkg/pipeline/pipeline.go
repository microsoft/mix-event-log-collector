package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	log "nuance.xaas-logging.event-log-collector/pkg/logging"
)

type PipelineComponent uint

const (
	FETCHER PipelineComponent = iota
	PROCESSOR
	WRITER
)

var (
	// Default pipeline config
	DefaultFetcherPath  = filepath.Join(os.TempDir(), "fetcher")
	DefaulProcessorPath = filepath.Join(os.TempDir(), "processor")
	DefaultPipelineYaml = PipelineYaml{
		Pipeline: PipelineParams{
			StorageProvider: "embedded",
			Fetcher: IOParams{
				Writer: &WriterParams{
					Path:             &DefaultFetcherPath,
					OperationTimeout: 30,
				},
			},
			Processor: IOParams{
				Reader: &ReaderParams{
					Path:                    &DefaultFetcherPath,
					AutoCommitOffsetEnabled: true,
					CreateUniqueGroupID:     true,
					OperationTimeout:        30,
				},
				Writer: &WriterParams{
					Path:             &DefaulProcessorPath,
					OperationTimeout: 30,
				},
			},
			Writer: IOParams{
				Reader: &ReaderParams{
					Path:                    &DefaulProcessorPath,
					AutoCommitOffsetEnabled: true,
					CreateUniqueGroupID:     true,
					OperationTimeout:        30,
				},
			},
		},
	}
)

// The following structs represent a pipeline yaml configuration structure
type PipelineYaml struct {
	Pipeline PipelineParams `yaml:"pipeline,omitempty"`
}

type PipelineParams struct {
	StorageProvider string   `yaml:"storage_provider"`
	Fetcher         IOParams `yaml:"fetcher,omitempty"`
	Processor       IOParams `yaml:"processor,omitempty"`
	Writer          IOParams `yaml:"writer,omitempty"`
}

type IOParams struct {
	Writer *WriterParams `yaml:"writer,omitempty"`
	Reader *ReaderParams `yaml:"reader,omitempty"`
}

type WriterParams struct {
	Path             *string  `yaml:"path,omitempty"`
	Host             *string  `yaml:"host,omitempty"`
	Hosts            []string `yaml:"hosts,omitempty"`
	Topic            *string  `yaml:"topic,omitempty"`
	OperationTimeout int      `yaml:"operation_timeout"`
	Password         string   `yaml:"password"`
	DB               int      `yaml:"DB"`
	Channel          string   `yaml:"channel"`
}

type ReaderParams struct {
	Path                    *string  `yaml:"path,omitempty"`
	Host                    *string  `yaml:"host,omitempty"`
	Hosts                   []string `yaml:"hosts,omitempty"`
	Group                   *string  `yaml:"group,omitempty"`
	CreateUniqueGroupID     bool     `yaml:"create_unique_group_id"`
	Topic                   *string  `yaml:"topic,omitempty"`
	Offset                  string   `yaml:"offset"`
	OperationTimeout        int      `yaml:"operation_timeout"`
	AutoCommitOffsetEnabled bool     `yaml:"auto_commit_offset_enabled"`
	Password                string   `yaml:"password"`
	DB                      int      `yaml:"DB"`
	Channel                 string   `yaml:"channel"`
}

// LoadPipelineConfigFromFile() loads the pipeline yaml config from file
func LoadPipelineConfigFromFile(configFile string) (*PipelineYaml, error) {
	// Start with defaults
	yml := DefaultPipelineYaml

	// Read config file content
	file, err := os.ReadFile(filepath.Clean(configFile))
	if err == nil {
		// Unmarshall yaml
		err = yaml.Unmarshal(file, &yml)
		if err != nil {
			return nil, err
		}
	}

	raw, _ := json.Marshal(log.MaskSensitiveData(yml))
	log.Debugf("%v", string(raw))

	return &yml, nil
}
