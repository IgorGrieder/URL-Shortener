package httpclient

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota + 1
	StateOpen
	StateHalfOpen
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type CircuitBreaker struct {
	mu          sync.Mutex
	state       State
	failures    int
	maxFailures int
	openSince   time.Time
	openTimeout time.Duration
}

func NewCircuitBreaker(maxFailures int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       StateClosed,
		maxFailures: maxFailures,
		openTimeout: openTimeout,
	}
}

func (cb *CircuitBreaker) CheckBeforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		if time.Since(cb.openSince) > cb.openTimeout {
			slog.Warn("Circuit Breaker: Open -> Half-Open")
			cb.state = StateHalfOpen
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		return ErrCircuitOpen
	}
	return nil
}

func (cb *CircuitBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		slog.Info("Circuit Breaker: Half-Open -> Closed")
		cb.state = StateClosed
		cb.failures = 0

	case StateClosed:
		cb.failures = 0
	}
}

func (cb *CircuitBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		slog.Error("Circuit Breaker: Half-Open -> Open (test failed)")
		cb.state = StateOpen
		cb.openSince = time.Now()

	case StateClosed:
		cb.failures++
		slog.Warn("Circuit Breaker: Failure recorded", "count", cb.failures)

		if cb.failures >= cb.maxFailures {
			slog.Error("Circuit Breaker: Closed -> Open (threshold reached)")
			cb.state = StateOpen
			cb.openSince = time.Now()
		}
	}
}
