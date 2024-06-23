package jobworker

import (
	"bufio"
	"fmt"
	"testing"
	"time"
)

func TestJobWorker_Can_Start_A_Job_Tail_Logs_Then_Stop_It(t *testing.T) {
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

	status, err := worker.Status(id)
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}

	if !status.Running {
		t.Error("expected job to be running and it isn't : ", err)
		return
	}

	reader, err := worker.Output(id)
	if err != nil {
		t.Error("could not get reader for job's output")
		return
	}

	// Assert output
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

	if err = worker.Stop(id); err != nil {
		t.Error("failed to stop job : ", err)
		return
	}

	time.Sleep(500 * time.Millisecond)

	status, err = worker.Status(id)
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}

	if status.Running {
		t.Error("expected job not to be running and it is")
		return
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

	time.Sleep(500 * time.Millisecond)

	status, err := worker.Status(id)
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}

	if status.Running {
		t.Error("expected job not to be running and it is")
		return
	}
}
