package example

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWorker_ProcessSingleJob(t *testing.T) {
	worker := NewWorker(10, 10)
	defer worker.Stop()

	// Submit job
	job := Job{ID: 1, Data: "test data"}
	if err := worker.SubmitJob(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Collect results
	var results []Result
	var mu sync.Mutex

	worker.GetResults().RunReceive(nil, func(result Result) {
		mu.Lock()
		results = append(results, result)
		mu.Unlock()
	})

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.JobID != job.ID {
		t.Errorf("Expected job ID %d, got %d", job.ID, result.JobID)
	}
	if result.Output != "processed: test data" {
		t.Errorf("Unexpected output: %s", result.Output)
	}
	if result.Error != nil {
		t.Errorf("Unexpected error: %v", result.Error)
	}
}

func TestWorker_ProcessMultipleJobs(t *testing.T) {
	worker := NewWorker(5, 5)
	defer worker.Stop()

	jobs := []Job{
		{ID: 1, Data: "job1"},
		{ID: 2, Data: "job2"},
		{ID: 3, Data: "job3"},
	}

	// Collect results
	var results []Result
	var mu sync.Mutex

	worker.GetResults().RunReceive(nil, func(result Result) {
		mu.Lock()
		results = append(results, result)
		mu.Unlock()
	})

	// Submit all jobs
	for _, job := range jobs {
		if err := worker.SubmitJob(job); err != nil {
			t.Fatalf("Failed to submit job %d: %v", job.ID, err)
		}
	}

	// Wait for all processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(results) != len(jobs) {
		t.Errorf("Expected %d results, got %d", len(jobs), len(results))
	}

	// Create map for easy verification
	resultMap := make(map[int]Result)
	for _, result := range results {
		resultMap[result.JobID] = result
	}

	// Verify all jobs were processed
	for _, job := range jobs {
		result, ok := resultMap[job.ID]
		if !ok {
			t.Errorf("Missing result for job %d", job.ID)
			continue
		}

		expectedOutput := "processed: " + job.Data
		if result.Output != expectedOutput {
			t.Errorf("Job %d: expected output %s, got %s",
				job.ID, expectedOutput, result.Output)
		}
	}
}

func TestWorker_FullInputBuffer(t *testing.T) {
	worker := NewWorker(2, 10) // Small input buffer
	defer worker.Stop()

	// Fill input buffer
	job1 := Job{ID: 1, Data: "job1"}
	job2 := Job{ID: 2, Data: "job2"}

	if err := worker.SubmitJob(job1); err != nil {
		t.Fatalf("Failed to submit first job: %v", err)
	}
	if err := worker.SubmitJob(job2); err != nil {
		t.Fatalf("Failed to submit second job: %v", err)
	}

	// Third job should fail (buffer full)
	job3 := Job{ID: 3, Data: "job3"}
	if err := worker.SubmitJob(job3); err == nil {
		t.Error("Expected error when submitting to full buffer")
	}

	// But blocking submit should work
	if err := worker.SubmitJobBlocking(job3); err != nil {
		t.Errorf("Blocking submit should work: %v", err)
	}
}

func TestWorker_Shutdown(t *testing.T) {
	worker := NewWorker(10, 10)

	// Submit job
	job := Job{ID: 1, Data: "test"}
	if err := worker.SubmitJob(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	worker.GetResults().RunReceive(nil, func(result Result) {})

	// Stop worker
	worker.Stop()

	// Wait for shutdown - longer timeout to account for close delay
	time.Sleep(1 * time.Second)

	// Verify worker is stopped
	if !worker.IsStopped() {
		t.Error("Worker should be stopped")
	}

	// Try to submit after shutdown (should fail)
	if err := worker.SubmitJob(Job{ID: 2, Data: "after shutdown"}); err == nil {
		t.Error("Expected error when submitting to stopped worker")
	}
}

func TestWorker_ConcurrentJobSubmission(t *testing.T) {
	worker := NewWorker(100, 100) // Larger buffer to handle concurrent load
	defer worker.Stop()

	// Collect results
	var results []Result
	var mu sync.Mutex
	var resultWg sync.WaitGroup

	worker.GetResults().RunReceive(nil, func(result Result) {
		mu.Lock()
		results = append(results, result)
		mu.Unlock()
		resultWg.Done()
	})

	// Submit jobs concurrently
	var submitWg sync.WaitGroup
	numWorkers := 5
	jobsPerWorker := 10
	totalJobs := numWorkers * jobsPerWorker

	// Set up result wait group
	resultWg.Add(totalJobs)

	for i := range numWorkers {
		submitWg.Add(1)
		go func(workerID int) {
			defer submitWg.Done()
			for j := range jobsPerWorker {
				job := Job{
					ID:   workerID*100 + j,
					Data: fmt.Sprintf("worker%d_job%d", workerID, j),
				}
				// Use blocking submit for reliability
				if err := worker.SubmitJobBlocking(job); err != nil {
					t.Errorf("Failed to submit job %d: %v", job.ID, err)
					return
				}
			}
		}(i)
	}

	submitWg.Wait()

	// Wait for all results to be processed
	resultWg.Wait()

	mu.Lock()
	defer mu.Unlock()

	expectedResults := numWorkers * jobsPerWorker
	if len(results) != expectedResults {
		t.Errorf("Expected %d results, got %d", expectedResults, len(results))
	}
}

func TestWorker_WithTimeout(t *testing.T) {
	worker := NewWorker(1, 1)
	defer worker.Stop()

	// Use context for timeout control
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	job := Job{ID: 1, Data: "test"}
	if err := worker.SubmitJob(job); err != nil {
		t.Fatalf("Failed to submit job: %v", err)
	}

	// Wait for result with timeout
	var result Result
	resultChan := make(chan Result, 1)

	worker.GetResults().RunReceive(nil, func(r Result) {
		resultChan <- r
	})

	select {
	case result = <-resultChan:
		// Success - verify result
		if result.JobID != job.ID {
			t.Errorf("Expected job ID %d, got %d", job.ID, result.JobID)
		}
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}
}

// Table-driven test for various inputs
func TestWorker_VariousInputs(t *testing.T) {
	tests := []struct {
		name     string
		input    Job
		expected string
	}{
		{
			name:     "normal job",
			input:    Job{ID: 1, Data: "normal"},
			expected: "processed: normal",
		},
		{
			name:     "empty data",
			input:    Job{ID: 2, Data: ""},
			expected: "processed: ",
		},
		{
			name:     "special characters",
			input:    Job{ID: 3, Data: "special!@#$%"},
			expected: "processed: special!@#$%",
		},
		{
			name:     "long data",
			input:    Job{ID: 4, Data: "this is a very long string that should be processed correctly"},
			expected: "processed: this is a very long string that should be processed correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new worker for each subtest to avoid interference
			worker := NewWorker(10, 10)
			defer worker.Stop()

			var result Result
			var mu sync.Mutex
			var received bool
			var wg sync.WaitGroup
			wg.Add(1)

			worker.GetResults().RunReceive(nil, func(r Result) {
				mu.Lock()
				if r.JobID == tt.input.ID {
					result = r
					received = true
					wg.Done()
				}
				mu.Unlock()
			})

			if err := worker.SubmitJob(tt.input); err != nil {
				t.Fatalf("Failed to submit job: %v", err)
			}

			// Wait for the specific result
			wg.Wait()

			mu.Lock()
			defer mu.Unlock()

			if !received {
				t.Fatal("No result received")
			}

			if result.Output != tt.expected {
				t.Errorf("Expected output %s, got %s", tt.expected, result.Output)
			}
		})
	}
}

// Benchmark worker performance
func BenchmarkWorker_JobProcessing(b *testing.B) {
	worker := NewWorker(1000, 1000)
	defer worker.Stop()

	// Start result consumer
	worker.GetResults().RunReceive(nil, func(result Result) {
		// Minimal processing
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			job := Job{ID: id, Data: "benchmark"}
			if err := worker.SubmitJob(job); err != nil {
				worker.SubmitJobBlocking(job)
			}
			id++
		}
	})
}
