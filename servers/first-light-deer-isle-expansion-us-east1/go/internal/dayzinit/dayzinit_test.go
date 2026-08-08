package dayzinit

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const validMission = `void main()
{
    Hive ce = CreateHive();
    if (ce)
        ce.InitOffline();
}

class CustomMission: MissionServer
{
    override PlayerBase CreateCharacter(PlayerIdentity identity, vector pos)
    {
        PlayerBase player;
        player = GetGame().CreatePlayer(identity, "Survivor", pos, 0, "NONE");
        return player;
    }
};

Mission CreateCustomMission(string path)
{
    return new CustomMission();
}
`

func TestValidateSourceAndErrorAPI(t *testing.T) {
	if err := ValidateSource("init.c", []byte(validMission)); err != nil {
		t.Fatalf("valid mission: %v", err)
	}

	err := ValidateSource("mission.c", []byte("void nope("))
	if err == nil {
		t.Fatal("invalid source returned nil")
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("errors.Is = false: %v", err)
	}
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("errors.As = false: %T", err)
	}
	if validationError.Path != "mission.c" || len(validationError.Diagnostics) < 2 {
		t.Fatalf("unexpected validation error: %#v", validationError)
	}
	message := validationError.Error()
	assertContains(t, message, "mission.c:1:1 [DZI1001]")
	assertContains(t, message, "hint:")

	var nilError *ValidationError
	if nilError.Error() != ErrInvalid.Error() {
		t.Fatalf("nil Error = %q", nilError.Error())
	}
	emptyError := (&ValidationError{Path: "init.c"}).Error()
	assertContains(t, emptyError, "invalid DayZ init.c")
	if !errors.Is((&ValidationError{}).Unwrap(), ErrInvalid) {
		t.Fatal("Unwrap did not return ErrInvalid")
	}
}

func TestValidateFileHandling(t *testing.T) {
	if err := ValidateFile(" "); err == nil {
		t.Fatal("empty path returned nil")
	}
	missing := filepath.Join(t.TempDir(), "init.c")
	if err := ValidateFile(missing); err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file error = %v", err)
	}
	if err := ValidateFile(t.TempDir()); err == nil {
		t.Fatal("directory returned nil")
	}

	path := filepath.Join(t.TempDir(), "INIT.C")
	data := append([]byte{0xef, 0xbb, 0xbf}, []byte(strings.ReplaceAll(validMission, "\n", "\r\n"))...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFile(path); err != nil {
		t.Fatalf("BOM CRLF file: %v", err)
	}

	readFailure := errors.New("read failure")
	err := validateFile("init.c", func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }, func(string) ([]byte, error) { return nil, readFailure })
	if !errors.Is(err, readFailure) {
		t.Fatalf("read error was not wrapped: %v", err)
	}
	if _, err := readRegularFile(missing); err == nil {
		t.Fatal("readRegularFile missing error = nil")
	}
	if _, err := readLimited(errorReader{}); err == nil {
		t.Fatal("readLimited error = nil")
	}
}

func TestSourceFailures(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		code string
	}{
		{name: "invalid UTF-8", data: []byte{0xff}, code: "DZI1003"},
		{name: "NUL", data: []byte("void\x00 main"), code: "DZI1004"},
		{name: "empty", data: []byte(" \r\n\t"), code: "DZI1005"},
		{name: "too large", data: make([]byte, maxSourceBytes+1), code: "DZI1002"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateSource(`C:\missions\init.c`, test.data)
			assertDiagnosticCode(t, err, test.code)
		})
	}
}

func TestValidateSourceStopsAfterFrontendErrors(t *testing.T) {
	assertDiagnosticCode(t, ValidateSource("init.c", []byte("/* unterminated")), "DZI1101")
	assertDiagnosticCode(t, ValidateSource("init.c", []byte("void main( {")), "DZI2105")
}

func TestConcurrentValidationAndDiagnosticIsolation(t *testing.T) {
	const count = 24
	var wait sync.WaitGroup
	errorsFound := make(chan error, count)
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsFound <- ValidateSource("init.c", []byte(validMission))
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatalf("concurrent validation: %v", err)
		}
	}

	first := validationError(t, ValidateSource("wrong.c", []byte("x")))
	first.Diagnostics[0].Message = "mutated"
	second := validationError(t, ValidateSource("wrong.c", []byte("x")))
	if second.Diagnostics[0].Message == "mutated" {
		t.Fatal("diagnostics leaked between calls")
	}
}

func TestPositionMappingAndPortableBase(t *testing.T) {
	source := newSourceFile("init.c", append([]byte{0xef, 0xbb, 0xbf}, []byte("aé\r\nb")...))
	position := source.position(6)
	if position.Line != 1 || position.Column != 3 || position.Offset != 6 {
		t.Fatalf("position = %#v", position)
	}
	position = source.position(8)
	if position.Line != 2 || position.Column != 1 {
		t.Fatalf("line two position = %#v", position)
	}
	if source.position(-1).Offset != 0 || source.position(999).Offset != len(source.data) {
		t.Fatal("position did not clamp offset")
	}
	if source.position(7).Column != 3 {
		t.Fatalf("CR column = %d", source.position(7).Column)
	}
	if portableBase(`C:\server\mpmissions\init.c`) != "init.c" || portableBase("/server/init.c") != "init.c" {
		t.Fatal("portableBase failed")
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "init.c" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestRepositoryMission(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "empty.deerisle", "init.c"))
	if err := ValidateFile(path); err != nil {
		t.Fatalf("repository init.c: %v", err)
	}
}

func TestOfficialCorpusWhenConfigured(t *testing.T) {
	directory := os.Getenv("DAYZINIT_OFFICIAL_CORPUS")
	if directory == "" {
		t.Skip("DAYZINIT_OFFICIAL_CORPUS is not set")
	}
	paths, err := filepath.Glob(filepath.Join(directory, "*", "init.c"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("official corpus paths: %v, count=%d", err, len(paths))
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			if err := ValidateFile(path); err != nil {
				t.Fatalf("official example: %v", err)
			}
		})
	}
}

func FuzzValidateSource(f *testing.F) {
	f.Add([]byte(validMission))
	f.Add([]byte{0xff, 0, '{', '}'})
	f.Fuzz(func(t *testing.T, source []byte) {
		_ = ValidateSource("init.c", source)
	})
}

func validationError(t *testing.T, err error) *ValidationError {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil")
	}
	var result *ValidationError
	if !errors.As(err, &result) {
		t.Fatalf("error type = %T", err)
	}
	return result
}

func assertDiagnosticCode(t *testing.T, err error, code string) {
	t.Helper()
	result := validationError(t, err)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostics %#v do not contain %s", result.Diagnostics, code)
}

func assertContains(t *testing.T, value, substring string) {
	t.Helper()
	if !strings.Contains(value, substring) {
		t.Fatalf("%q does not contain %q", value, substring)
	}
}
