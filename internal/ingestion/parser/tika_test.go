package parser

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTikaParser_Success(t *testing.T) {
	var capturedBody []byte
	var capturedCT string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		capturedCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		capturedBody = b
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Extracted PDF text\n"))
	}))
	defer server.Close()

	p := NewTikaDocumentParser(server.URL, 5*time.Second)
	res, err := p.Parse(context.Background(), []byte("%PDF-1.4 fake"), "application/pdf", "doc.pdf")
	require.NoError(t, err)
	assert.Equal(t, "Extracted PDF text", res.Text)
	assert.Equal(t, "application/pdf", capturedCT)
	assert.Equal(t, []byte("%PDF-1.4 fake"), capturedBody)
}

func TestTikaParser_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		_, _ = w.Write([]byte("unsupported"))
	}))
	defer server.Close()

	p := NewTikaDocumentParser(server.URL, 5*time.Second)
	_, err := p.Parse(context.Background(), []byte("xxx"), "application/x-bogus", "doc.bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "415")
}

func TestTikaParser_Supports(t *testing.T) {
	p := TikaDocumentParser{}
	assert.True(t, p.Supports("application/pdf"))
	assert.True(t, p.Supports("application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
	assert.False(t, p.Supports("text/plain"))
	assert.False(t, p.Supports("text/markdown"))
}
