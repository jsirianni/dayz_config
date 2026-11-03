package logreader

import (
	"encoding/json"
	"strings"
	"time"
)

// Record is one captured subprocess output line. Level identifies the stream,
// not a severity inferred from the message. Timestamp is the UTC capture time.
type Record struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func DecodeRecord(line string) (Record, bool) {
	var fields map[string]json.RawMessage
	var record Record
	if json.Unmarshal([]byte(line), &fields) != nil || len(fields) != 3 || fields["message"] == nil || string(fields["message"]) == "null" || fields["level"] == nil || fields["timestamp"] == nil {
		return record, false
	}
	if json.Unmarshal([]byte(line), &record) != nil || (record.Level != "info" && record.Level != "error") || record.Timestamp.IsZero() {
		return Record{}, false
	}
	return record, true
}

func messageText(line string) string {
	if record, ok := DecodeRecord(line); ok {
		return record.Message
	}
	return line
}

func encodeRecord(record Record) string {
	b, _ := json.Marshal(record)
	return string(b)
}

// prepareMessages examines decoded text too, so JSON escapes cannot conceal key
// boundaries when a bounded tail begins in the middle of a private-key block.
func (r *redactor) prepareMessages(text string) {
	var messages strings.Builder
	for _, line := range strings.Split(text, "\n") {
		messages.WriteString(messageText(line))
		messages.WriteByte('\n')
	}
	r.prepare(messages.String())
}
