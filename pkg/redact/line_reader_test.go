package redact

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1.2 & 1.3: NewLineReader creates instance correctly
func TestNewLineReader(t *testing.T) {
	input := strings.NewReader("test")
	lr := NewLineReader(input)

	require.NotNil(t, lr)
	require.NotNil(t, lr.reader)
}

// Test 1.8: Empty file → (nil, false, io.EOF)
func TestLineReader_EmptyFile(t *testing.T) {
	lr := NewLineReader(strings.NewReader(""))

	line, hadNewline, err := lr.ReadLine()

	assert.Nil(t, line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Test 1.9: Single line with \n → (content, true, nil)
func TestLineReader_SingleLineWithNewline(t *testing.T) {
	lr := NewLineReader(strings.NewReader("hello world\n"))

	line, hadNewline, err := lr.ReadLine()

	assert.Equal(t, []byte("hello world"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Second read should return EOF
	line, hadNewline, err = lr.ReadLine()
	assert.Nil(t, line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Test 1.10: Single line without \n → (content, false, io.EOF)
func TestLineReader_SingleLineWithoutNewline(t *testing.T) {
	lr := NewLineReader(strings.NewReader("hello world"))

	line, hadNewline, err := lr.ReadLine()

	assert.Equal(t, []byte("hello world"), line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Test 1.11: Multiple lines with \n → correct for each
func TestLineReader_MultipleLinesWithNewlines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	lr := NewLineReader(strings.NewReader(input))

	// First line
	line, hadNewline, err := lr.ReadLine()
	assert.Equal(t, []byte("line1"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Second line
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte("line2"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Third line
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte("line3"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// EOF
	line, hadNewline, err = lr.ReadLine()
	assert.Nil(t, line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Test 1.12: Last line without \n → (content, false, io.EOF)
func TestLineReader_LastLineWithoutNewline(t *testing.T) {
	input := "line1\nline2\nline3"
	lr := NewLineReader(strings.NewReader(input))

	// First line
	line, hadNewline, err := lr.ReadLine()
	assert.Equal(t, []byte("line1"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Second line
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte("line2"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Third line (no trailing newline)
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte("line3"), line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Test 1.13: Binary data (no \n) → (all content, false, io.EOF)
func TestLineReader_BinaryData(t *testing.T) {
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0xFF, 0xFE}
	lr := NewLineReader(bytes.NewReader(binaryData))

	line, hadNewline, err := lr.ReadLine()

	assert.Equal(t, binaryData, line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Test 1.14: Line exceeding max size → error
func TestLineReader_LineExceedingMaxSize(t *testing.T) {
	// Create a line that exceeds SCANNER_MAX_SIZE
	largeData := make([]byte, constants.SCANNER_MAX_SIZE+100)
	for i := range largeData {
		largeData[i] = 'a'
	}

	lr := NewLineReader(bytes.NewReader(largeData))

	line, hadNewline, err := lr.ReadLine()

	assert.Nil(t, line)
	assert.False(t, hadNewline)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")
}

// Test 1.15: File with only \n → ([], true, nil)
func TestLineReader_OnlyNewline(t *testing.T) {
	lr := NewLineReader(strings.NewReader("\n"))

	line, hadNewline, err := lr.ReadLine()

	assert.Equal(t, []byte{}, line) // Empty line
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Second read should return EOF
	line, hadNewline, err = lr.ReadLine()
	assert.Nil(t, line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Additional test: File with empty lines (multiple newlines)
func TestLineReader_EmptyLines(t *testing.T) {
	input := "\n\n\n"
	lr := NewLineReader(strings.NewReader(input))

	// First empty line
	line, hadNewline, err := lr.ReadLine()
	assert.Equal(t, []byte{}, line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Second empty line
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte{}, line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Third empty line
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte{}, line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// EOF
	line, hadNewline, err = lr.ReadLine()
	assert.Nil(t, line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Additional test: Mixed content with and without newlines
func TestLineReader_MixedContent(t *testing.T) {
	input := "line1\n\nline3"
	lr := NewLineReader(strings.NewReader(input))

	// First line
	line, hadNewline, err := lr.ReadLine()
	assert.Equal(t, []byte("line1"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Empty line
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte{}, line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Last line without newline
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte("line3"), line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Additional test: Large but valid file (under max size)
func TestLineReader_LargeValidFile(t *testing.T) {
	// Create a line that's large but under the limit
	largeData := make([]byte, constants.SCANNER_MAX_SIZE-100)
	for i := range largeData {
		largeData[i] = 'x'
	}
	largeData = append(largeData, '\n')

	lr := NewLineReader(bytes.NewReader(largeData))

	line, hadNewline, err := lr.ReadLine()

	assert.Equal(t, constants.SCANNER_MAX_SIZE-100, len(line))
	assert.True(t, hadNewline)
	assert.NoError(t, err)
}

// Additional test: Binary file with embedded newlines
func TestLineReader_BinaryWithEmbeddedNewlines(t *testing.T) {
	binaryData := []byte{0x01, 0x02, '\n', 0x03, 0x04, '\n', 0x05}
	lr := NewLineReader(bytes.NewReader(binaryData))

	// First "line" (up to first \n)
	line, hadNewline, err := lr.ReadLine()
	assert.Equal(t, []byte{0x01, 0x02}, line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Second "line"
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte{0x03, 0x04}, line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	// Last "line" without newline
	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte{0x05}, line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Test edge case: Very small reads
func TestLineReader_SingleByteReads(t *testing.T) {
	input := "a\nb\nc"
	lr := NewLineReader(strings.NewReader(input))

	line, hadNewline, err := lr.ReadLine()
	assert.Equal(t, []byte("a"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte("b"), line)
	assert.True(t, hadNewline)
	assert.NoError(t, err)

	line, hadNewline, err = lr.ReadLine()
	assert.Equal(t, []byte("c"), line)
	assert.False(t, hadNewline)
	assert.Equal(t, io.EOF, err)
}

// Benchmark: LineReader vs bufio.Scanner performance
func BenchmarkLineReader(b *testing.B) {
	// Create test data
	var buf bytes.Buffer
	for i := 0; i < 1000; i++ {
		buf.WriteString("This is line number ")
		buf.WriteString(string(rune(i)))
		buf.WriteString(" with some content\n")
	}
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lr := NewLineReader(bytes.NewReader(data))
		for {
			_, _, err := lr.ReadLine()
			if err == io.EOF {
				break
			}
		}
	}
}

