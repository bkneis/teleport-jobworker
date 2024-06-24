package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"os/user"
	"syscall"

	"github.com/teleport-jobworker/pkg/jobworker"
)

// cleanup is called after capturing Ctrl+C and used to delete the started job
func cleanup(id string, worker *jobworker.JobWorker) {
	if id != "" {
		err := worker.Stop(id)
		if err != nil {
			log.Printf("could not stop job %s", id)
			log.Fatal(err)
			return
		}
		log.Printf("Stopped job %s", id)
	}
}

// todo move to examples folder in root and move pkg/jobworker to root
func main() {

	var err error
	var id string

	worker := jobworker.New()

	// Set current user as owner
	user, err := user.Current()
	if err != nil {
		log.Print(err)
		return
	}

	// Define job's command and options
	cmd := os.Args[1]
	args := os.Args[2:]
	opts := jobworker.NewOpts(100, 100, 50)

	// Run the job
	id, err = worker.Start(opts, user.Name, cmd, args...)
	if err != nil {
		log.Print("failed to start command")
		log.Print(err)
		return
	}

	// Capture Ctrl+C and stop job if started
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func(i string, w *jobworker.JobWorker) {
		<-c
		cleanup(i, w)
		os.Exit(1)
	}(id, worker)

	status, err := worker.Status(id)
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
	reader, err := worker.Output(id)
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
