# Channel Encapsulation

A thread-safe, generic channel wrapper for Go that provides graceful closure and enhanced safety features.

## Why Encapsulation?

### Problems with Native Go Channels

1. **Race Conditions**: Direct channel operations can lead to race conditions when checking closure state
2. **Panic on Closed Channels**: Sending to a closed channel causes panic
3. **Resource Management**: No built-in graceful shutdown mechanisms
4. **Error Handling**: Limited error feedback for failed operations

### Our Solution

This encapsulated channel provides:
- **Thread-safe operations** with mutex protection
- **Graceful closure** with configurable delays
- **Error handling** instead of panics
- **Resource cleanup** with proper lifecycle management
- **Type safety** with Go generics

## Design Principles

### 1. Safety First
```go
// Thread-safe closure check
func (c *Channel[T]) Closed() bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.closed
}
```

### 2. Graceful Shutdown
```go
// Close with delay allows processing of remaining messages
channel.Close(5 * time.Second)
```

### 3. Error Handling
```go
// Returns errors instead of panicking
if err := channel.Send(data); err != nil {
    log.Printf("Send failed: %v", err)
}
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "fmt"
    "time"
    "your-project/channel"
)

func main() {
    // Create a buffered channel
    ch := channel.NewChannel[string](10)
    
    // Start receiving messages
    ch.RunReceive(
        func() { fmt.Println("Panic recovered") }, // Recovery handler
        func(data string) { fmt.Println("Received:", data) }, // Callback
    )
    
    // Send messages
    if err := ch.Send("Hello"); err != nil {
        fmt.Printf("Send error: %v\n", err)
    }
    
    // Blocking send (waits if channel is full)
    if err := ch.SendWithStuck("World"); err != nil {
        fmt.Printf("SendWithStuck error: %v\n", err)
    }
    
    // Graceful shutdown after 2 seconds
    ch.Close(2 * time.Second)
}
```

### Advanced Usage: Message Processing System

```go
type MessageProcessor struct {
    input  *channel.Channel[Message]
    output *channel.Channel[ProcessedMessage]
}

func NewMessageProcessor() *MessageProcessor {
    mp := &MessageProcessor{
        input:  channel.NewChannel[Message](100),
        output: channel.NewChannel[ProcessedMessage](50),
    }
    
    // Start processing pipeline
    mp.input.RunReceive(
        func() { log.Println("Input processor recovered from panic") },
        mp.processMessage,
    )
    
    return mp
}

func (mp *MessageProcessor) processMessage(msg Message) {
    // Process the message
    processed := ProcessedMessage{
        ID:        msg.ID,
        Content:   strings.ToUpper(msg.Content),
        Timestamp: time.Now(),
    }
    
    // Forward to output channel
    if err := mp.output.Send(processed); err != nil {
        log.Printf("Failed to send processed message: %v", err)
    }
}

func (mp *MessageProcessor) Shutdown() {
    mp.input.Close(5 * time.Second)  // Allow 5 seconds for processing
    mp.output.Close(1 * time.Second) // Quick shutdown for output
}
```

### Error Handling Patterns

```go
func robustSender(ch *channel.Channel[int], data int) {
    // Try non-blocking send first
    if err := ch.Send(data); err != nil {
        if err.Error() == "channel is full" {
            // Channel full, try blocking send
            if err := ch.SendWithStuck(data); err != nil {
                log.Printf("Failed to send data %d: %v", data, err)
                return
            }
        } else {
            // Channel closed
            log.Printf("Channel closed, cannot send data %d", data)
            return
        }
    }
    log.Printf("Successfully sent data: %d", data)
}
```

## API Reference

### Constructor

#### `NewChannel[T any](size int) *Channel[T]`
Creates a new channel with the specified buffer size.
- `size`: Buffer size (negative values become 0)

### Methods

#### `Send(value T) error`
Non-blocking send. Returns error if channel is full or closed.

#### `SendWithStuck(value T) error`
Blocking send. Waits if channel is full, returns error if closed.

#### `RunReceive(recoverHandler func(), callbacks ...func(data T))`
Starts a goroutine to receive messages and execute callbacks.
- `recoverHandler`: Called when panic occurs (can be nil)
- `callbacks`: Functions to execute for each received message

#### `Close(after time.Duration) error`
Initiates graceful closure after specified delay.
- `after`: Duration to wait before closing

#### `Closed() bool`
Thread-safe check if channel is closed.

## Architecture Benefits

### 1. Encapsulation
- Hides complex channel management logic
- Provides clean, intuitive API
- Prevents misuse of low-level channel operations

### 2. Composability
- Easy to integrate into larger systems
- Supports multiple callbacks for fan-out patterns
- Works well with dependency injection

### 3. Testability
- Clear error boundaries
- Predictable behavior
- Easy to mock for unit tests

### 4. Performance
- Minimal overhead over native channels
- Efficient mutex usage (RWMutex for reads)
- Buffered control channel prevents blocking

## Best Practices

1. **Always handle errors** from `Send()` and `SendWithStuck()`
2. **Use appropriate buffer sizes** based on your use case
3. **Implement panic recovery** in `RunReceive()` callbacks
4. **Allow sufficient time** for graceful shutdown
5. **Check `Closed()` state** before critical operations

## Thread Safety

All operations are thread-safe:
- Multiple goroutines can safely call `Send()` concurrently
- `Closed()` can be called from any goroutine
- `Close()` is protected by `sync.Once` to prevent multiple closures
- The `closed` flag is protected by `sync.RWMutex`

This encapsulation transforms Go's powerful but sometimes dangerous channels into a robust, production-ready communication primitive.