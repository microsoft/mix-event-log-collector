package utils

import (
	"encoding/json"
	"errors"
	"strings"
)

type TruncatedEventLogSchema struct {
	Key *EventLogKey `json:"key,omitempty"`
}

type EventLogKey struct {
	Service string `json:"service,omitempty"`
	ID      string `json:"id,omitempty"`
}

// PreProcessRedactedFields() is a utility function to fix the redacted fields in the NLUaaS event log
func PreProcessRedactedFields(in []byte) ([]byte, error) {
	// Need to check for redacted fields in the pipeline layer because some pipeline providers (e.g. kafka) need to
	// be able to deserialize the event log record into a struct. If the redacted fields are not fixed, the
	// deserialization will fail.

	// Extract event log key
	var key TruncatedEventLogSchema
	err := json.Unmarshal(in, &key)
	if err != nil {
		return in, err
	}

	// Fix redacted fields
	switch {
	case key.Key == nil:
		return in, errors.New("event log key is nil")
	case key.Key.Service == "NLUaaS":
		record := string(in)
		if strings.Contains(record, `"interpretations":"****redacted****"`) {
			record = strings.Replace(record,
				`"interpretations":"****redacted****"`,
				`"interpretations":["literal": "****redacted****"]`, -1)
		}
		return []byte(record), nil
	default:
		return in, nil
	}
}
