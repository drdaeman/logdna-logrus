package logdna

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"
)

// Most of this code is taken from logrus.TextFormatter source,
// https://github.com/sirupsen/logrus/blob/master/text_formatter.go
// with some adaptations (mostly, code removal).
//
// Logrus is copyrighted by Simon Eskildsen and licensed under MIT license.
// For more details see https://github.com/sirupsen/logrus/blob/master/LICENSE

// SimpleTextFormatter is a simplified version of logrus.TextFormatter
// It only renders the message text followed with key-value pairs,
// no colors (and terminal detection) or timestamps.
type SimpleTextFormatter struct {
	DisableSorting   bool
	QuoteEmptyFields bool
}

// Format renders a single log entry
func (f *SimpleTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
	}

	if !f.DisableSorting {
		sort.Strings(keys)
	}

	fmt.Fprintf(b, "%s ", entry.Message)
	for _, k := range keys {
		v := entry.Data[k]
		fmt.Fprintf(b, " %s=", k)
		f.appendValue(b, v)
	}
	return b.Bytes(), nil
}

func (f *SimpleTextFormatter) needsQuoting(text string) bool {
	if f.QuoteEmptyFields && len(text) == 0 {
		return true
	}
	for _, ch := range text {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '.' || ch == '_' ||
			ch == '/' || ch == '@' || ch == '^' || ch == '+') {
			return true
		}
	}
	return false
}

func (f *SimpleTextFormatter) appendValue(b *bytes.Buffer, value interface{}) {
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}

	if !f.needsQuoting(stringVal) {
		b.WriteString(stringVal)
	} else {
		b.WriteString(fmt.Sprintf("%q", stringVal))
	}
}
