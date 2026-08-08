package dayzinit

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ValidateFile reads and validates a DayZ server mission init.c file.
func ValidateFile(filename string) error {
	return validateFile(filename, os.Stat, readRegularFile)
}

func validateFile(filename string, stat func(string) (os.FileInfo, error), read func(string) ([]byte, error)) error {
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("DayZ init.c path is empty")
	}
	info, err := stat(filename)
	if err != nil {
		return fmt.Errorf("stat %s: %w", filename, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", filename)
	}
	data, err := read(filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}
	return ValidateSource(filename, data)
}

func readRegularFile(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readLimited(file)
}

func readLimited(reader io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, maxSourceBytes+1))
}

// ValidateSource validates already-loaded source using the same rules as
// ValidateFile. filename is used in diagnostics.
func ValidateSource(filename string, data []byte) error {
	source := newSourceFile(filename, data)
	var found diagnostics

	if !strings.EqualFold(portableBase(filename), "init.c") {
		found.add(Diagnostic{
			Code:    "DZI1001",
			Message: "expected a DayZ mission file named init.c",
			Hint:    "validate the mission entry file named init.c",
			Span:    source.span(0, 0),
		})
	}
	if len(data) > maxSourceBytes {
		found.add(Diagnostic{Code: "DZI1002", Message: fmt.Sprintf("source exceeds the %d-byte safety limit", maxSourceBytes), Span: source.span(0, 0)})
		return validationResult(filename, &found)
	}
	if offset := firstInvalidUTF8(data); offset >= 0 {
		found.add(Diagnostic{Code: "DZI1003", Message: "source is not valid UTF-8", Hint: "save init.c as UTF-8 with or without a BOM", Span: source.span(offset, offset+1)})
		return validationResult(filename, &found)
	}
	if offset := strings.IndexByte(string(data), 0); offset >= 0 {
		found.add(Diagnostic{Code: "DZI1004", Message: "source contains an embedded NUL byte", Hint: "remove the NUL byte", Span: source.span(offset, offset+1)})
		return validationResult(filename, &found)
	}
	text := data[source.bomBytes:]
	if strings.TrimSpace(string(text)) == "" {
		found.add(Diagnostic{Code: "DZI1005", Message: "init.c is empty or contains only whitespace", Hint: "add the DayZ mission entry points", Span: source.span(source.bomBytes, source.bomBytes)})
		return validationResult(filename, &found)
	}

	variants, preprocessorDiagnostics := analyzePreprocessor(source)
	found.merge(preprocessorDiagnostics)
	if len(preprocessorDiagnostics) == 0 {
		for _, variant := range variants {
			tokens, lexicalDiagnostics := lex(source, variant)
			found.merge(lexicalDiagnostics)
			if len(lexicalDiagnostics) != 0 {
				continue
			}
			program, parseDiagnostics := parse(source, tokens)
			found.merge(parseDiagnostics)
			if len(parseDiagnostics) != 0 {
				continue
			}
			found.merge(validateSemantics(source, program))
			found.merge(validateMissionContract(source, program))
		}
	}
	return validationResult(filename, &found)
}

func validationResult(filename string, found *diagnostics) error {
	items := found.sorted()
	if len(items) == 0 {
		return nil
	}
	return &ValidationError{Path: filename, Diagnostics: items}
}

func portableBase(filename string) string {
	return path.Base(filepath.ToSlash(strings.ReplaceAll(filename, `\`, "/")))
}
