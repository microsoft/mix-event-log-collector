package processor

/*
	Base filter implementation for custom filters to inherit from
*/

import "strings"

const (
	EventAsrCallSummary = "application/x-nuance-asr-callsummary."
)

// The Filter takes an array of mix event log content data types which can be used to filter out
// event log data not required for particular reporting and analytics. This is especially useful in
// speeding up log processing and reducing storage requirements.
type Filter struct {
	config           FilterConfig
	ignoreAsrSummary bool
}

// NewFilter() creates a new instance of a Filter
func NewFilter(config FilterConfig) *Filter {
	f := &Filter{
		config: config,
	}
	// If asr-callsummary event logs are to be filtered, set this flag so that processors
	// can determine if merging of asr-recognitioninitmesage and asr-finalstatusresponse
	// event logs into asr-callsummary event logs is necessary
	f.ignoreAsrSummary = f.IsFiltered(EventAsrCallSummary)
	return f
}

// IsAsrSummaryIgnored returns true if asr-callsummary event logs can be ignored
func (f *Filter) IsAsrSummaryIgnored() bool {
	return f.ignoreAsrSummary
}

// applyWhiteListFilter() returns true if the event log data content type is in the filter list
func (f *Filter) applyWhiteListFilter(input string) bool {
	for _, value := range f.config.ContentType {
		switch strings.HasSuffix(value, "*") {
		case true:
			if strings.HasPrefix(input, strings.TrimSuffix(value, "*")) {
				return false
			}
		case false:
			if input == value {
				return false
			}
		}
	}
	return true
}

// applyBlackListFilter() returns false if the event log data content type is in the filter list
func (f *Filter) applyBlackListFilter(input string) bool {
	for _, value := range f.config.ContentType {
		switch strings.HasSuffix(value, "*") {
		case true:
			if strings.HasPrefix(input, strings.TrimSuffix(value, "*")) {
				return true
			}
		case false:
			if input == value {
				return true
			}
		}
	}
	return false
}

// IsFiltered() returns true if the event log data content type meets the filter criteria
func (f *Filter) IsFiltered(input string) bool {
	switch f.config.Type {
	case "whitelist":
		return f.applyWhiteListFilter(input)
	default:
		return f.applyBlackListFilter(input)

	}
}
