package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/teleport-jobworker/pkg/jobworker"
)

// Example usage: ./example_concurrent NUM_CLIENTS bash -c "while true; do echo hello; sleep 0.2; done"
func main() {
	var err error
	if len(os.Args) < 3 {
		fmt.Print(`Not enough arguments, usage: ./example_concurrent 20 bash -c "echo hello"`)
		return
	}
	// Define job's command and options
	opts := jobworker.JobOpts{100, 50, 100 * jobworker.CgroupMB}
	// Run the job
	job, err := jobworker.Start(opts, os.Args[2], os.Args[3:]...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Capture Ctrl+C and stop job
	wg := &sync.WaitGroup{}
	wg.Add(1)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func(j *jobworker.Job, w *sync.WaitGroup) {
		<-c
		defer wg.Done()
		if err := job.Stop(ctx); err != nil {
			fmt.Print(err)
			return
		}
		fmt.Printf("Stopped job %s\n", job.ID)
	}(job, wg)

	// Check the status
	status := job.Status()
	// Check it's running
	fmt.Println(status)
	if !status.Running {
		fmt.Print("job not running")
		return
	}
	// Parse number of clients to simulate
	numClients, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Print("example provided invalid number")
		return
	}

	for i := range numClients {
		// Get io.ReadCloser tailing job logs
		reader, err := job.Output(jobworker.FollowLogs)
		if err != nil {
			fmt.Print("could not get reader for job's output")
			return
		}
		defer reader.Close()

		// Print logs to STDOUT
		fmt.Println("Job logs")
		go func(r io.ReadCloser, n int) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Printf("Reader %d: %s\n", n, line)
			}
		}(reader, i)
	}

	// Wait for Ctrl+C
	wg.Wait()
	os.Exit(1)
}
