package jobworker

import (
	"bufio"
	"fmt"
	"testing"
	"time"
)

func TestJobWorker_Can_Start_A_Job_And_Tail_Logs_Then_Stop_It(t *testing.T) {
	worker := &JobWorker{
		jobs: JobsList{},
		con:  &Cgroup{"/tmp"},
	}

	n := 5
	echo := "hello"
	cmd := fmt.Sprintf("for run in {1..%d}; do echo %s; sleep 0.1; done", n, echo)
	opts := NewOpts(100, 100, 50)

	// Run the job
	id, err := worker.Start(opts, "TEST", cmd)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	// Check the status and it's running
	status, err := worker.Status(id)
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}

	if !status.Running {
		t.Error("expected job to be running and it isn't : ", err)
		return
	}

	// Assert output
	reader, err := worker.Output(id)
	if err != nil {
		t.Error("could not get reader for job's output")
		return
	}
	scanner := bufio.NewScanner(reader)
	i := 1
	for scanner.Scan() {
		if i >= n {
			break
		}
		i += 1
		line := scanner.Text()
		if line != "hello" {
			t.Errorf("log output was not as expected, actual %s expected %s", line, echo)
		}
	}

	// Stop the job
	if err = worker.Stop(id); err != nil {
		t.Error("failed to stop job : ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status, err = worker.Status(id)
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}
	if status.Running {
		t.Error("expected job not to be running and it is")
		return
	}
	// Assert exit code -1 since we sent a signal to terminate the job
	if status.ExitCode != -1 {
		t.Errorf("expected exit code to be -1 and was %d", status.ExitCode)
	}
}

func TestJobWorker_Status_After_Job_Completes(t *testing.T) {
	worker := &JobWorker{
		jobs: JobsList{},
		con:  &Cgroup{"/tmp"},
	}

	cmd := "echo hello world"
	opts := NewOpts(100, 100, 50)

	// Run the job
	id, err := worker.Start(opts, "TEST", cmd)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status, err := worker.Status(id)
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}
	if status.Running {
		t.Error("expected job not to be running and it is")
		return
	}
	if status.ExitCode != 0 {
		t.Errorf("expected exit code to be 0 and was %d", status.ExitCode)
	}
}

func TestJobWorker_Exit_Code_Is_Propagated(t *testing.T) {
	worker := &JobWorker{
		jobs: JobsList{},
		con:  &Cgroup{"/tmp"},
	}

	cmd := "exit 4"
	opts := NewOpts(100, 100, 50)

	// Run the job
	id, err := worker.Start(opts, "TEST", cmd)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status, err := worker.Status(id)
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}
	if status.Running {
		t.Error("expected job not to be running and it is")
		return
	}
	// Assert exit code 4 was set in status
	if status.ExitCode != 4 {
		t.Errorf("expected exit code to be 4 and was %d", status.ExitCode)
	}
}
