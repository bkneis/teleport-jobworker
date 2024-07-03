package jobworker

import (
	"bufio"
	"context"
	"fmt"
	"io"
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
	args := []string{"-c", fmt.Sprintf("for run in {1..%d}; do echo \\${run} %s; sleep 0.1; done", n, echo)}
	opts := JobOpts{100, 50, 100 * CgroupMB}
	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
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
				// todo fix
				fmt.Println(logs)
			}
			// todo fix assertion
			for _, log := range logs {
				if log != echo {
					t.Errorf("actual %s was not expected %s", log, echo)
				}
			}
		}(reader)
	}

	if err := job.Stop(ctx); err != nil {
		t.Errorf("failed to stop job: %v", err)
		return
	}
}
