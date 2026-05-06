package aiclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripMarkdownCodeFence_JSONFence(t *testing.T) {
	in := "```json\n{\"a\": 1}\n```"
	assert.Equal(t, `{"a": 1}`, StripMarkdownCodeFence(in))
}

func TestStripMarkdownCodeFence_NoLanguage(t *testing.T) {
	in := "```\nhello\n```"
	assert.Equal(t, "hello", StripMarkdownCodeFence(in))
}

func TestStripMarkdownCodeFence_PythonFence(t *testing.T) {
	in := "```python\nprint('hi')\n```"
	assert.Equal(t, "print('hi')", StripMarkdownCodeFence(in))
}

func TestStripMarkdownCodeFence_NoFence_ReturnsAsIs(t *testing.T) {
	in := `{"a": 1}`
	assert.Equal(t, in, StripMarkdownCodeFence(in))
}

func TestStripMarkdownCodeFence_Empty(t *testing.T) {
	assert.Equal(t, "", StripMarkdownCodeFence(""))
}

func TestStripMarkdownCodeFence_TrimsOuterWhitespace(t *testing.T) {
	in := "  \n```json\n{\"a\":1}\n```  \n"
	assert.Equal(t, `{"a":1}`, StripMarkdownCodeFence(in))
}

func TestStripMarkdownCodeFence_OnlyLeadingFence(t *testing.T) {
	in := "```json\n{\"a\":1}"
	assert.Equal(t, `{"a":1}`, StripMarkdownCodeFence(in))
}

func TestStripMarkdownCodeFence_OnlyTrailingFence(t *testing.T) {
	in := "{\"a\":1}\n```"
	assert.Equal(t, `{"a":1}`, StripMarkdownCodeFence(in))
}
