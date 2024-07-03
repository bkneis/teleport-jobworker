/*
Package jobworker implements executing arbitrary linux commands as "jobs" with functions to manage the job and it's underlying linux process
as well as specify resource controls for the process.

The Start function initiates a job and runs the linux command as an exec.Cmd in addition to a go routine to call the blocking function
waiting to process the exit code. By default jobworker will use cgroups v2 to manage resource control of jobs using values in JobOpts.

JobOpts allows three resource controllers to be configured, CPU, memory and IO. CPU is controlled using a weight, as defined by cgroups
cpu.weight interface file, memory is a soft limit specified as mem.high interface file and IO also using io.weight.

Additionally other implementations of resource control can be used by implementing ResourceController and use with StartWithController.

Start returns a jobworker.Job that allows the caller to stop, get the status or tail it's logs.

Example usage of executing a linux command and tailing it's output. Note when calling Output with follow=true, the caller needs
to unblock the reading go routine by calling Close, in this example we call Close after catching Ctrl+C

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

    // Get the status
    status := job.Status()
    if !status.Running {
        log.Error("job not running")
        return
    }

    // Get io.ReadCloser to tail job's output
    reader, err := job.Output(jobworker.FollowLogs)
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
