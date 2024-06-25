package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/teleport-jobworker/pkg/jobworker"
)

var NUM_CLIENTS = 100

// Example usage: ./example_concurrent bash -c "while true; do echo hello; sleep 0.2; done"
func main() {
	var err error
	if len(os.Args) < 2 {
		fmt.Print(`Not enough arguments, usage: ./example bash -c "echo hello"`)
		return
	}
	// Define job's command and options
	opts := jobworker.NewOpts(100, 50, 100*jobworker.CgroupMB)
	// Run the job
	job, err := jobworker.Start(opts, os.Args[1], os.Args[2:]...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return
	}

	// Capture Ctrl+C and stop job
	wg := &sync.WaitGroup{}
	wg.Add(1)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func(j *jobworker.Job, w *sync.WaitGroup) {
		<-c
		defer wg.Done()
		if err := job.Stop(); err != nil {
			fmt.Print(err)
			return
		}
		fmt.Printf("Stopped job %s\n", job.ID)
	}(job, wg)

	// Check the status
	status, err := job.Status()
	if err != nil {
		fmt.Print("failed to get status")
		fmt.Print(err)
		return
	}
	// Check it's running
	fmt.Println(status)
	if !status.Running {
		fmt.Print("job not running")
		return
	}

	for _ = range NUM_CLIENTS {
		// Get io.ReadCloser tailing job logs
		reader, err := job.Output()
		if err != nil {
			fmt.Print("could not get reader for job's output")
			return
		}
		defer reader.Close()

		// Print logs to STDOUT
		fmt.Println("Job logs")
		go func(r io.ReadCloser) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				_ = scanner.Text()
			}
		}(reader)
	}

	// time.Sleep(10 * time.Second)

	// if err := job.Stop(); err != nil {
	// 	fmt.Print(err)
	// 	return
	// }
	// fmt.Printf("Stopped job %s\n", job.ID)

	// Wait for Ctrl+C
	wg.Wait()
	os.Exit(1)
}
