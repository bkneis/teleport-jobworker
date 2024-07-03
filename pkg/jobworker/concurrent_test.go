package jobworker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

const numClients = 10

func TestConcurrentReaders(t *testing.T) {
	WORKER_UID = -1
	WORKER_GUID = -1
	// Define number of log iterations and content
	n := 5
	echo := "hello"
	// Define job's command and options for test
	cmd := "bash"
	args := []string{"-c", fmt.Sprintf("for run in {1..%d}; do echo ${run}: %s; sleep 0.01; done", n, echo)}
	opts := JobOpts{100, 50, 100 * CgroupMB}
	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Spawn multiple clients to read log output
	for _ = range numClients {
		// Get io.ReadCloser tailing job logs
		reader, err := job.Output(FollowLogs)
		if err != nil {
			t.Errorf("could not get reader for job's output: %v", err)
			return
		}
		defer reader.Close()

		// Read logs and assert contents and length
		go func(r io.ReadCloser) {
			scanner := bufio.NewScanner(r)
			logs := []string{}
			for scanner.Scan() {
				logs = append(logs, scanner.Text())
				if len(logs) >= n {
					break
				}
			}
			// todo fix assertion
			for i, log := range logs {
				expected := fmt.Sprintf("%d: %s", i+1, echo)
				if log != expected {
					t.Errorf("actual %s was not expected %s", log, expected)
				}
			}
		}(reader)
	}

	<-ctx.Done()
}

// TestConcurrentReadersNoFollow tests the same as above but not following the logs
func TestConcurrentReadersNoFollow(t *testing.T) {
	WORKER_UID = -1
	WORKER_GUID = -1
	// Define number of log iterations and content
	n := 5
	echo := "hello"
	// Define job's command and options for test
	cmd := "bash"
	args := []string{"-c", fmt.Sprintf("for run in {1..%d}; do echo ${run}: %s; sleep 0.01; done", n, echo)}
	opts := JobOpts{100, 50, 100 * CgroupMB}
	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return
	}

	wg := &sync.WaitGroup{}

	// Spawn multiple clients to read log output
	for _ = range numClients {
		// Get io.ReadCloser tailing job logs
		reader, err := job.Output(DontFollowLogs)
		if err != nil {
			t.Errorf("could not get reader for job's output: %v", err)
			return
		}
		defer reader.Close()

		// Read logs and assert contents and length
		wg.Add(1)
		go func(r io.ReadCloser, w *sync.WaitGroup) {
			defer w.Done()
			scanner := bufio.NewScanner(r)
			logs := []string{}
			for scanner.Scan() {
				logs = append(logs, scanner.Text())
			}
			// todo fix assertion
			for i, log := range logs {
				expected := fmt.Sprintf("%d: %s", i+1, echo)
				if log != expected {
					t.Errorf("actual %s was not expected %s", log, expected)
				}
			}
		}(reader, wg)
	}

	wg.Wait()
}
