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
			log.Fatal(err)
			return
		}
		log.Printf("Stopped job %s", id)
	}
}

func main() {

	var err error
	var id string

	worker := jobworker.New()

	// Capture Ctrl+C and stop job
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanup(id, worker)
		os.Exit(1)
	}()

	user, err := user.Current()
	if err != nil {
		log.Print(err)
		return
	}

	// Define job's command and options
	cmd := "while true; do echo hello; sleep 2; done"
	opts := jobworker.NewOpts(100, 100, 50)

	// Run the job
	id, err = worker.Start(opts, user.Name, cmd)
	if err != nil {
		log.Print("failed to start command")
		log.Print(err)
		return
	}

	// log.Printf("Job created with ID %s", id)

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
