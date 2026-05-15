package device

import (
	"fmt"
	"sync"
)

type AckResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type commandAckRegistry struct {
	mu      sync.Mutex
	waiters map[string]chan AckResult
}

func newCommandAckRegistry() *commandAckRegistry {
	return &commandAckRegistry{waiters: make(map[string]chan AckResult)}
}

func (r *commandAckRegistry) Register(requestID string) (<-chan AckResult, func(), error) {
	if requestID == "" {
		return nil, nil, fmt.Errorf("request id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.waiters[requestID]; exists {
		return nil, nil, fmt.Errorf("ack waiter already registered for request id %s", requestID)
	}

	ch := make(chan AckResult, 1)
	r.waiters[requestID] = ch
	release := func() {
		r.mu.Lock()
		delete(r.waiters, requestID)
		r.mu.Unlock()
	}
	return ch, release, nil
}

func (r *commandAckRegistry) TryDeliver(requestID string, res AckResult) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch, exists := r.waiters[requestID]
	if !exists {
		return false
	}

	delete(r.waiters, requestID)
	select {
	case ch <- res:
	default:
	}
	return true
}
