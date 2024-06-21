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

func cleanup(id string, worker *jobworker.JobWorker) {
	if id != "" {
		err := worker.Stop(id)
		if err != nil {
			log.Fatal(err)
			return
		}
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

	cmd := "while true; do echo hello; sleep 2; done"
	opts := jobworker.NewOpts(100, "100M", "50ms")

	id, err = worker.Start(opts, user.Name, cmd)
	if err != nil {
		log.Print(err)
		return
	}

	status := worker.Status(id)
	if err != nil {
		log.Print(err)
		return
	}

	if !status.Running {
		log.Print("job not running")
		return
	}

	reader, err := worker.Output(id)
	if err != nil {
		log.Print("could not get reader for job's output")
		return
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("%s\n", line)
	}
}
