package ssh

import (
	"context"
	"sync"
	"time"
)

// MockExecutor 用于测试
type MockExecutor struct {
	mu      sync.Mutex
	scripts map[string]MockResult // key: command
}

type MockResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
	DelayMs  int
}

func NewMockExecutor() *MockExecutor { return &MockExecutor{scripts: map[string]MockResult{}} }

func (m *MockExecutor) Set(cmd string, res MockResult) {
	m.mu.Lock()
	m.scripts[cmd] = res
	m.mu.Unlock()
}

func (m *MockExecutor) Exec(ctx context.Context, user, addr, key, cmd string, timeout time.Duration) (string, string, int, error) {
	m.mu.Lock()
	r, ok := m.scripts[cmd]
	m.mu.Unlock()
	if !ok {
		return "", "", 127, nil
	}
	if r.DelayMs > 0 {
		select {
		case <-ctx.Done():
			return "", "", -1, ctx.Err()
		case <-time.After(time.Duration(r.DelayMs) * time.Millisecond):
		}
	}
	return r.Stdout, r.Stderr, r.ExitCode, r.Err
}
