/*
Package jobworker implements executing arbitrary linux commands as "jobs" with functions to manage the job and it's underlying linux process
as well as specify resource controls for the process.

The Start function initiates a job and runs the linux command as an exec.Cmd in addition to a go routine to call the blocking function
waiting to process the exit code. By default jobworker will use cgroups v2 to manage resource control of jobs using values in JobOpts.

Additionally other implementations of resource control can be used by implementing ResourceController and use with StartWithController.

Start returns a jobworker.Job that allows the caller to Stop, Status and Stop the job.

Example usage of executing a linux command and tailing it's output

package main

import (
    "log"
    "bufio"
    "os/user"
    "github.com/bkneis/teleport-jobworker/pkg/jobworker"
)

func main() {

    cmd := "bash"
    args := []string{"-c", `"while true; do echo hello; sleep 2; done"`}

    // Start the job
    job, err := jobworker.Start(JobOpts{100, 100 * jobworker.CgroupMB, 50}, cmd, args)
    if err != nil {
        log.Error(err)
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

    // Get the status
    status := job.Status()
    if !status.Running {
        log.Error("job not running")
        return
    }

    // Get io.ReadCloser to tail job's output
    reader, err := job.Output()
    if err != nil {
        log.Error("could not get reader for job's output")
        return
    }
	// Make sure we close the tailReader to ensure logging go routine exits cleanly
    defer reader.Close()

	// Log job output to STDOUT
    go func(r io.ReadCloser) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("%s\n", line)
		}
	}(reader)

	wg.Wait()
}
*/
package jobworker
