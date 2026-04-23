package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocumentStatusConstants(t *testing.T) {
	assert.Equal(t, DocumentStatus("pending"), DocStatusPending)
	assert.Equal(t, DocumentStatus("running"), DocStatusRunning)
	assert.Equal(t, DocumentStatus("success"), DocStatusSuccess)
	assert.Equal(t, DocumentStatus("failed"), DocStatusFailed)
}

func TestNormalizeSourceType(t *testing.T) {
	cases := []struct {
		input    string
		expected SourceType
		hasErr   bool
	}{
		{"file", SourceTypeFile, false},
		{"FILE", SourceTypeFile, false},
		{"localfile", SourceTypeFile, false},
		{"local_file", SourceTypeFile, false},
		{"url", SourceTypeURL, false},
		{"URL", SourceTypeURL, false},
		{"", SourceTypeFile, false}, // 空串默认 file
		{"ftp", "", true},
	}

	for _, c := range cases {
		got, err := NormalizeSourceType(c.input)
		if c.hasErr {
			assert.Error(t, err, "input=%q", c.input)
		} else {
			assert.NoError(t, err, "input=%q", c.input)
			assert.Equal(t, c.expected, got, "input=%q", c.input)
		}
	}
}

func TestNormalizeProcessMode(t *testing.T) {
	cases := []struct {
		input    string
		expected ProcessMode
		hasErr   bool
	}{
		{"chunk", ProcessModeChunk, false},
		{"CHUNK", ProcessModeChunk, false},
		{"pipeline", ProcessModePipeline, false},
		{"", ProcessModeChunk, false},
		{"foobar", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeProcessMode(c.input)
		if c.hasErr {
			assert.Error(t, err, "input=%q", c.input)
		} else {
			assert.NoError(t, err, "input=%q", c.input)
			assert.Equal(t, c.expected, got, "input=%q", c.input)
		}
	}
}
