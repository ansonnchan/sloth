package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

// Output keeps machine data and human diagnostics on separate streams.
type Output struct {
	Data        io.Writer
	Diagnostics io.Writer
	Quiet       bool
}

// WriteJSON emits one undecorated JSON value to stdout.
func (o Output) WriteJSON(value any) error {
	return json.NewEncoder(o.Data).Encode(value)
}

// Progressf emits nonessential progress unless quiet mode is active.
func (o Output) Progressf(format string, args ...any) error {
	if o.Quiet {
		return nil
	}
	_, err := fmt.Fprintf(o.Diagnostics, format, args...)
	return err
}

// Warningf emits a warning even in quiet mode.
func (o Output) Warningf(format string, args ...any) error {
	_, err := fmt.Fprintf(o.Diagnostics, format, args...)
	return err
}
