package batchvalidate

import (
	"strings"
	"testing"
)

func BenchmarkValidateSmall(b *testing.B) {
	src := []byte("@echo off\nif exist file (echo yes) else (echo no)\n")
	for index := 0; index < b.N; index++ {
		ValidateSource("small.cmd", src, Options{})
	}
}

func BenchmarkValidateLargeLinear(b *testing.B) {
	src := []byte(strings.Repeat("echo a >out && echo b\n", 1000))
	for index := 0; index < b.N; index++ {
		ValidateSource("large.cmd", src, Options{})
	}
}

func BenchmarkValidateLabelsAndGotos(b *testing.B) {
	var source strings.Builder
	for index := 0; index < 1000; index++ {
		source.WriteString(":label\ngoto label\n")
	}
	src := []byte(source.String())
	for index := 0; index < b.N; index++ {
		ValidateSource("labels.cmd", src, Options{})
	}
}

func BenchmarkValidateDeepNesting(b *testing.B) {
	src := []byte(strings.Repeat("(", 64) + "echo x" + strings.Repeat(")", 64))
	for index := 0; index < b.N; index++ {
		ValidateSource("deep.cmd", src, Options{})
	}
}

func BenchmarkValidateLargeArithmetic(b *testing.B) {
	src := []byte("set /a \"A=" + strings.Repeat("1+", 1000) + "1\"")
	for index := 0; index < b.N; index++ {
		ValidateSource("arithmetic.cmd", src, Options{})
	}
}

func BenchmarkValidateExternalCommands(b *testing.B) {
	src := []byte(strings.Repeat("vendor.exe --flag=value >out\n", 1000))
	for index := 0; index < b.N; index++ {
		ValidateSource("external.cmd", src, Options{})
	}
}
