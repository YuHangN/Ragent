package ingestion

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Node is the interface every pipeline step implements.
type Node interface {
	Name() string
	Execute(ctx context.Context, ic *IngestionContext) NodeResult
}

// Pipeline runs a fixed sequence of nodes left to right.
type Pipeline struct {
	nodes []Node
}

// NewPipeline creates a pipeline from an ordered list of nodes.
func NewPipeline(nodes ...Node) *Pipeline {
	return &Pipeline{nodes: nodes}
}

// Run executes each node in order, stopping on failure or Terminate.
func (p *Pipeline) Run(ctx context.Context, ic *IngestionContext) error {
	if ic.Logs == nil {
		ic.Logs = make([]NodeLog, 0, len(p.nodes))
	}

	for _, node := range p.nodes {
		start := time.Now()
		result := node.Execute(ctx, ic)
		ms := time.Since(start).Milliseconds()

		log := NodeLog{
			Node:       node.Name(),
			Message:    result.Message,
			DurationMs: ms,
			Success:    result.Success,
		}
		if result.Err != nil {
			log.Error = result.Err.Error()
		}
		ic.Logs = append(ic.Logs, log)

		zap.S().Infof("ingestion node %s: %s (%dms)", node.Name(), result.Message, ms)

		if !result.Success {
			ic.Status = "failed"
			ic.Error = result.Err
			return fmt.Errorf("node %s failed: %w", node.Name(), result.Err)
		}
		if !result.ShouldContinue {
			break
		}
	}

	if ic.Status != "failed" {
		ic.Status = "success"
	}
	return nil
}
