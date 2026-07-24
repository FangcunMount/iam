package readiness

import (
	"context"
	"sync/atomic"
	"time"
)

type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
	StatusFailed   Status = "failed"
	StatusSkipped  Status = "skipped"
)

type ComponentResult struct {
	Status Status `json:"status"`
}

type Snapshot struct {
	Status     string                     `json:"status"`
	Components map[string]ComponentResult `json:"components"`
	CheckedAt  time.Time                  `json:"checked_at"`
}

type CheckFunc func(context.Context) error

type Component struct {
	Name     string
	Required bool
	Check    CheckFunc
}

type Config struct {
	ComponentTimeout time.Duration
	TotalTimeout     time.Duration
}

type Checker struct {
	config     Config
	components []Component
	draining   atomic.Bool
}

func New(config Config, components []Component) *Checker {
	return &Checker{config: config, components: components}
}

func (c *Checker) MarkDraining() {
	if c != nil {
		c.draining.Store(true)
	}
}

func (c *Checker) Check(ctx context.Context) (Snapshot, bool) {
	componentCount := 0
	if c != nil {
		componentCount = len(c.components)
	}
	snapshot := Snapshot{
		Status:     "ready",
		Components: make(map[string]ComponentResult, componentCount),
		CheckedAt:  time.Now().UTC(),
	}
	if c == nil || c.draining.Load() {
		snapshot.Status = "not_ready"
		return snapshot, false
	}

	totalCtx, cancel := context.WithTimeout(ctx, c.config.TotalTimeout)
	defer cancel()
	type result struct {
		name     string
		required bool
		err      error
		status   Status
	}
	results := make(chan result, len(c.components))
	for _, component := range c.components {
		component := component
		go func() {
			if component.Check == nil {
				results <- result{name: component.Name, required: component.Required, status: StatusSkipped}
				return
			}
			componentCtx, componentCancel := context.WithTimeout(totalCtx, c.config.ComponentTimeout)
			defer componentCancel()
			err := component.Check(componentCtx)
			results <- result{name: component.Name, required: component.Required, err: err}
		}()
	}

	ready := true
	received := make(map[string]struct{}, len(c.components))
	for range c.components {
		select {
		case item := <-results:
			received[item.name] = struct{}{}
			status := item.status
			if status == "" {
				status = StatusOK
				if item.err != nil {
					status = StatusDegraded
					if item.required {
						status = StatusFailed
						ready = false
					}
				}
			}
			snapshot.Components[item.name] = ComponentResult{Status: status}
		case <-totalCtx.Done():
			ready = false
			for _, component := range c.components {
				if _, ok := received[component.Name]; ok {
					continue
				}
				status := StatusDegraded
				if component.Required {
					status = StatusFailed
				}
				snapshot.Components[component.Name] = ComponentResult{Status: status}
			}
			snapshot.Status = "not_ready"
			return snapshot, false
		}
	}
	if !ready {
		snapshot.Status = "not_ready"
	}
	return snapshot, ready
}
