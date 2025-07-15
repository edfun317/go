package channel

import (
	"errors"
	"sync"
	"time"
)

type (
	// Channel is a generic channel wrapper that provides safe operations and graceful closure
	Channel[T any] struct {
		ch     chan T          // Main data channel
		close  chan closeAfter // Control channel for closure signals
		closed bool            // Flag indicating if channel is closed
		mu     sync.RWMutex    // Mutex to protect closed field from race conditions
		once   sync.Once       // Ensures close operation happens only once
	}

	// closeAfter represents a close signal with optional delay
	closeAfter struct {
		d time.Duration // Duration to wait before closing
	}
)

// NewChannel creates a new Channel instance with the specified buffer size
// Returns an error if size is negative
func NewChannel[T any](size int) *Channel[T] {
	if size < 0 {
		size = 0
	}

	return &Channel[T]{
		ch:    make(chan T, size),
		close: make(chan closeAfter, 1), // Buffered to prevent blocking
	}
}

// SendWithStuck sends a value to the channel, blocking if the channel is full
// Returns an error if the channel is closed
func (c *Channel[T]) SendWithStuck(value T) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return errors.New(ErrChannelClosed)
	}

	c.ch <- value
	return nil
}

// RunReceive starts a goroutine to continuously receive values from the channel
// and execute the provided callbacks. Handles panic recovery and graceful shutdown.
func (c *Channel[T]) RunReceive(recoverHandler func(), callbacks ...func(data T)) {

	go func() {
		defer func() {
			if r := recover(); r != nil {
				if recoverHandler != nil {

					recoverHandler()
				}
			}
		}()

		for {
			select {
			// Handle close signal
			case t := <-c.close:
				c.once.Do(func() {
					// Wait for specified duration before closing
					if t.d > 0 {
						timer := time.NewTimer(t.d)
						<-timer.C
					}

					// Thread-safe closure
					c.mu.Lock()
					c.closed = true
					c.mu.Unlock()

					close(c.ch)
				})
				return

			// Handle incoming data
			case value, ok := <-c.ch:
				if !ok {
					return // Channel closed
				}

				// Execute all callbacks with the received value
				for _, callback := range callbacks {
					if callback != nil {
						callback(value)
					}
				}
			}
		}
	}()
}

// Send attempts to send a value to the channel without blocking
// Returns an error if the channel is closed or full
func (c *Channel[T]) Send(value T) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return errors.New(ErrChannelClosed)
	}

	select {
	case c.ch <- value:
		return nil
	default:
		return errors.New("channel is full")
	}
}

// Close initiates graceful closure of the channel after the specified duration
// Returns an error if the channel is already closed or if close signal cannot be sent
func (c *Channel[T]) Close(after time.Duration) error {

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return errors.New(ErrChannelClosed)
	}

	select {
	case c.close <- closeAfter{d: after}:
		return nil
	default:
		return errors.New("close channel is full")
	}
}

// Closed returns true if the channel is closed in a thread-safe manner
func (c *Channel[T]) Closed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}
