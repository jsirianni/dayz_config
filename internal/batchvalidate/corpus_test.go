package batchvalidate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureCorpus(t *testing.T) {
	categories := []struct {
		directory   string
		wantErrors  bool
		wantPartial bool
	}{
		{"valid", false, false},
		{"invalid", true, false},
		{"ambiguous", false, true},
		{"opaque", false, true},
		{"corpus", false, true},
	}
	for _, category := range categories {
		entries, err := os.ReadDir(filepath.Join("testdata", category.directory))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) == ".md" {
				continue
			}
			t.Run(category.directory+"/"+entry.Name(), func(t *testing.T) {
				path := filepath.Join("testdata", category.directory, entry.Name())
				result, err := ValidateFile(path, Options{InitialCommandExtensions: FeatureEnabled})
				if err != nil {
					t.Fatal(err)
				}
				if result.HasErrors() != category.wantErrors {
					t.Fatalf("diagnostics = %#v, want errors %v", result.Diagnostics, category.wantErrors)
				}
				if category.wantPartial && result.FullyValidated {
					t.Fatalf("fixture was unexpectedly fully validated: %#v", result)
				}
			})
		}
	}
}

func TestGeneratedCombinations(t *testing.T) {
	conditions := []string{"errorlevel 1", "A==A", "exist file", "1 EQU 1", "cmdextversion 1", "defined NAME"}
	prefixes := []string{"", "not "}
	bodies := []string{"echo yes", "(echo yes)"}
	for _, condition := range conditions {
		for _, prefix := range prefixes {
			for _, body := range bodies {
				src := "if " + prefix + condition + " " + body
				result := ValidateSource("generated-if.cmd", []byte(src), Options{InitialCommandExtensions: FeatureEnabled})
				if result.HasErrors() {
					t.Errorf("%q diagnostics = %#v", src, result.Diagnostics)
				}
			}
		}
	}
	for _, handle := range []string{"", "0", "1", "2", "9"} {
		for _, operator := range []string{">", ">>", "<"} {
			src := "echo x " + handle + operator + "target"
			if result := ValidateSource("generated-redirection.cmd", []byte(src), Options{}); result.HasErrors() {
				t.Errorf("%q diagnostics = %#v", src, result.Diagnostics)
			}
		}
	}
	for _, mode := range []string{"", "/d ", "/r ", "/l "} {
		set := "(*)"
		if mode == "/l " {
			set = "(1,1,2)"
		}
		src := "for " + mode + "%%A in " + set + " do echo %%A"
		if result := ValidateSource("generated-for.cmd", []byte(src), Options{InitialCommandExtensions: FeatureEnabled}); result.HasErrors() {
			t.Errorf("%q diagnostics = %#v", src, result.Diagnostics)
		}
	}
}
