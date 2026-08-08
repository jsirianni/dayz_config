package dayzinit

import (
	"sort"
	"unicode/utf8"
)

const maxSourceBytes = 8 << 20

type sourceFile struct {
	name       string
	data       []byte
	lineStarts []int
	bomBytes   int
}

func newSourceFile(name string, data []byte) *sourceFile {
	source := &sourceFile{name: name, data: data, lineStarts: []int{0}}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		source.bomBytes = 3
		source.lineStarts[0] = 3
	}
	for offset := source.bomBytes; offset < len(data); offset++ {
		if data[offset] == '\n' {
			source.lineStarts = append(source.lineStarts, offset+1)
		}
	}
	return source
}

func (source *sourceFile) position(offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source.data) {
		offset = len(source.data)
	}
	lineIndex := sort.Search(len(source.lineStarts), func(index int) bool {
		return source.lineStarts[index] > offset
	}) - 1
	if lineIndex < 0 {
		lineIndex = 0
	}
	lineStart := source.lineStarts[lineIndex]
	if offset < lineStart {
		return Position{Offset: offset, Line: lineIndex + 1, Column: 1}
	}
	columnBytes := source.data[lineStart:offset]
	if len(columnBytes) > 0 && columnBytes[len(columnBytes)-1] == '\r' {
		columnBytes = columnBytes[:len(columnBytes)-1]
	}
	return Position{Offset: offset, Line: lineIndex + 1, Column: utf8.RuneCount(columnBytes) + 1}
}

func (source *sourceFile) span(start, end int) Span {
	return Span{Start: source.position(start), End: source.position(end)}
}

func firstInvalidUTF8(data []byte) int {
	for offset := 0; offset < len(data); {
		_, size := utf8.DecodeRune(data[offset:])
		if size == 1 && data[offset] >= utf8.RuneSelf {
			return offset
		}
		offset += size
	}
	return -1
}
