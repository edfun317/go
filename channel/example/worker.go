package example

import (
	"time"

	"github.com/edfun317/go/channel"
)

// Worker demonstrates a typical use case for the channel
type Worker struct {
	input  *channel.Channel[Job]
	output *channel.Channel[Result]
}

// Job represents a work item
type Job struct {
	ID   int
	Data string
}

// Result represents the processed result
type Result struct {
	JobID     int
	Output    string
	Error     error
	Timestamp time.Time
}

// NewWorker creates a new worker with specified buffer sizes
func NewWorker(inputBuffer, outputBuffer int) *Worker {
	w := &Worker{
		input:  channel.NewChannel[Job](inputBuffer),
		output: channel.NewChannel[Result](outputBuffer),
	}

	// Start worker goroutine
	w.input.RunReceive(
		func() { /* panic recovery */ },
		w.processJob,
	)

	return w
}

// processJob processes a single job
func (w *Worker) processJob(job Job) {
	// Simulate work
	time.Sleep(10 * time.Millisecond)

	result := Result{
		JobID:     job.ID,
		Output:    "processed: " + job.Data,
		Error:     nil,
		Timestamp: time.Now(),
	}

	// Send result (non-blocking)
	if err := w.output.Send(result); err != nil {
		// If output channel is full, try blocking send
		w.output.SendWithStuck(result)
	}
}

// SubmitJob submits a job for processing
func (w *Worker) SubmitJob(job Job) error {
	return w.input.Send(job)
}

// SubmitJobBlocking submits a job with blocking if input is full
func (w *Worker) SubmitJobBlocking(job Job) error {
	return w.input.SendWithStuck(job)
}

// GetResults returns the output channel for consuming results
func (w *Worker) GetResults() *channel.Channel[Result] {
	return w.output
}

// Stop gracefully shuts down the worker
func (w *Worker) Stop() {
	w.input.Close(100 * time.Millisecond)
	w.output.Close(50 * time.Millisecond)
}

// IsStopped checks if the worker is stopped
func (w *Worker) IsStopped() bool {
	return w.input.Closed() && w.output.Closed()
}