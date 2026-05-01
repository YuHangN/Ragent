package aiclient

import (
	"context"
)

type ChatClient interface {
	Provider() Provider
	Chat(ctx context.Context, req ChatRequest, target *ModelTarget) (string, error)
	StreamChat(ctx context.Context, req ChatRequest, cb StreamCallback, target *ModelTarget) error
}
