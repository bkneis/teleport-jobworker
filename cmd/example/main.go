package main

import (
	"bufio"
	"log"
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
			log.Printf("could not stop job %s", id)
			log.Fatal(err)
			return
		}
		log.Printf("Stopped job %s", job.ID)
	}
}

// todo move to examples folder in root and move pkg/jobworker to root
func main() {

	var err error
	var id string

	// Define job's command and options
	cmd := os.Args[1]
	args := os.Args[2:]
	opts := jobworker.NewOpts(100, 100, 50)

	// Run the job
	job, err := jobworker.Start(opts, cmd, args...)
	if err != nil {
		log.Print("failed to start command")
		log.Print(err)
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

	status, err := job.Status()
	if err != nil {
		log.Print("failed to get status")
		log.Print(err)
		return
	}

	log.Print(status)
	if !status.Running {
		log.Print("job not running")
		return
	}

	// Get io.ReadCloser tailing job logs
	reader, err := job.Output()
	if err != nil {
		log.Print("could not get reader for job's output")
		return
	}

	// Log output to STDOUT
	log.Print("Job's logs")
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("%s\n", line)
	}
}
