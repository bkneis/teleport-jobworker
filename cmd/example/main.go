package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/teleport-jobworker/pkg/jobworker"
)

// cleanup is called after capturing Ctrl+C and used to delete the started job
func cleanup(id string, job *jobworker.Job) {
	if job != nil {
		err := job.Stop()
		if err != nil {
			fmt.Printf("could not stop job %s", id)
			fmt.Print(err)
			return
		}
		fmt.Printf("Stopped job %s", job.ID)
	}
}

// Example usage: ./example bash -c "echo hello"
func main() {
	var err error
	var id string
	// Define job's command and options
	cmd := os.Args[1]
	args := os.Args[2:]
	opts := jobworker.NewOpts(100, 50, 100*jobworker.CgroupMB)
	// Run the job
	job, err := jobworker.Start(opts, cmd, args...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return
	}
	// Capture Ctrl+C and stop job if started
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func(i string, j *jobworker.Job) {
		<-c
		cleanup(i, j)
		os.Exit(1)
	}(id, job)
	// Check the status
	status, err := job.Status()
	if err != nil {
		fmt.Print("failed to get status")
		fmt.Print(err)
		return
	}
	// Check is running
	fmt.Println(status)
	if !status.Running {
		fmt.Print("job not running")
		return
	}
	// Get io.ReadCloser tailing job logs
	reader, err := job.Output()
	if err != nil {
		fmt.Print("could not get reader for job's output")
		return
	}
	defer reader.Close()

	// Log output to STDOUT
	fmt.Println("Job logs")
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("%s\n", line)
	}
}
