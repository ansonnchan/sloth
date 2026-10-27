package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
)

const MaxReportBytes int64 = 16 << 20

type ReadErrorKind string

const (
	ReadMissing          ReadErrorKind = "missing"
	ReadUnreadable       ReadErrorKind = "unreadable"
	ReadOversized        ReadErrorKind = "oversized"
	ReadMalformed        ReadErrorKind = "malformed"
	ReadTruncated        ReadErrorKind = "truncated"
	ReadUnsupportedMajor ReadErrorKind = "unsupported_major"
	ReadValidation       ReadErrorKind = "validation"
)

type ReadError struct {
	Kind ReadErrorKind
	Path string
	Err  error
}

func (err *ReadError) Error() string {
	if err.Path == "" {
		return fmt.Sprintf("read report (%s): %v", err.Kind, err.Err)
	}
	return fmt.Sprintf("read report %q (%s): %v", err.Path, err.Kind, err.Err)
}

func (err *ReadError) Unwrap() error { return err.Err }

// Parsed keeps compatible unknown top-level fields so a read/write workflow
// can preserve forward extensions even when this binary does not understand them.
type Parsed struct {
	Report        Report
	UnknownFields map[string]json.RawMessage
	NewerMinor    bool
}

func Read(path string) (Parsed, error) {
	file, err := os.Open(path)
	if err != nil {
		kind := ReadUnreadable
		if errors.Is(err, os.ErrNotExist) {
			kind = ReadMissing
		}
		return Parsed{}, &ReadError{Kind: kind, Path: path, Err: err}
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Parsed{}, &ReadError{Kind: ReadUnreadable, Path: path, Err: err}
	}
	if info.Size() > MaxReportBytes {
		return Parsed{}, &ReadError{Kind: ReadOversized, Path: path, Err: fmt.Errorf("size %d exceeds %d-byte limit", info.Size(), MaxReportBytes)}
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxReportBytes+1))
	if err != nil {
		return Parsed{}, &ReadError{Kind: ReadUnreadable, Path: path, Err: err}
	}
	parsed, err := Parse(data)
	if readErr := new(ReadError); errors.As(err, &readErr) {
		readErr.Path = path
	}
	return parsed, err
}

func Parse(data []byte) (Parsed, error) {
	if int64(len(data)) > MaxReportBytes {
		return Parsed{}, &ReadError{Kind: ReadOversized, Err: fmt.Errorf("size %d exceeds %d-byte limit", len(data), MaxReportBytes)}
	}
	var raw map[string]json.RawMessage
	if err := decodeOne(data, &raw); err != nil {
		return Parsed{}, &ReadError{Kind: classifyJSONError(err), Err: err}
	}
	var version string
	if err := json.Unmarshal(raw["schema_version"], &version); err != nil {
		return Parsed{}, &ReadError{Kind: ReadValidation, Err: errors.New("schema_version must be a string")}
	}
	major, minor, err := parseSchemaVersion(version)
	if err != nil {
		return Parsed{}, &ReadError{Kind: ReadValidation, Err: err}
	}
	if major != 1 {
		return Parsed{}, &ReadError{Kind: ReadUnsupportedMajor, Err: fmt.Errorf("schema major v%d is unsupported; this binary supports v1", major)}
	}

	var value Report
	if err := json.Unmarshal(data, &value); err != nil {
		return Parsed{}, &ReadError{Kind: classifyJSONError(err), Err: err}
	}
	validated := value
	validated.SchemaVersion = SchemaVersion
	if err := Validate(validated); err != nil {
		return Parsed{}, &ReadError{Kind: ReadValidation, Err: err}
	}

	unknown := make(map[string]json.RawMessage)
	for key, encoded := range raw {
		if _, known := knownReportFields[key]; !known {
			unknown[key] = append(json.RawMessage(nil), encoded...)
		}
	}
	return Parsed{Report: value, UnknownFields: unknown, NewerMinor: minor > 0}, nil
}

func decodeOne(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

var schemaVersionPattern = regexp.MustCompile(`^sloth\.report/v([0-9]+)\.([0-9]+)$`)

func parseSchemaVersion(value string) (int, int, error) {
	matches := schemaVersionPattern.FindStringSubmatch(value)
	if matches == nil {
		return 0, 0, fmt.Errorf("unsupported schema_version %q", value)
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	return major, minor, nil
}

func classifyJSONError(err error) ReadErrorKind {
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return ReadTruncated
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) && syntax.Offset > 0 {
		if text := err.Error(); text == "unexpected end of JSON input" {
			return ReadTruncated
		}
	}
	return ReadMalformed
}

var knownReportFields = map[string]struct{}{
	"schema_version": {}, "tool_version": {}, "rule_version": {}, "tool": {}, "run": {},
	"target": {}, "configuration": {}, "environment": {}, "overall": {}, "experiments": {},
	"diagnostics": {}, "limitations": {}, "samples": {},
}
