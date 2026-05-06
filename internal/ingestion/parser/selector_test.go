package parser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserSelector_SelectByMime(t *testing.T) {
	s := NewDocumentParserSelector([]DocumentParser{
		NewTextDocumentParser(),
		NewMarkdownDocumentParser(),
	})

	p, err := s.SelectByMime("text/plain")
	require.NoError(t, err)
	assert.Equal(t, ParserTypeText, p.Type())

	p, err = s.SelectByMime("text/markdown")
	require.NoError(t, err)
	assert.Equal(t, ParserTypeMarkdown, p.Type())

	_, err = s.SelectByMime("application/x-bogus")
	require.Error(t, err)
}

func TestParserSelector_Parse(t *testing.T) {
	s := NewDocumentParserSelector([]DocumentParser{
		NewTextDocumentParser(),
	})
	res, err := s.Parse(context.Background(), []byte("hello"), "text/plain", "a.txt")
	require.NoError(t, err)
	assert.Equal(t, "hello", res.Text)
}
