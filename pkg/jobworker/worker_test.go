package jobworker

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// mockController implements ResourceController but stubs out the implementation so the file system
// is not manipulated for unit tests
type mockController struct{}

func (con *mockController) AddProcess(name string, cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{CgroupFD: -1}
	return nil
}
func (con *mockController) CreateGroup(name string) error                      { return nil }
func (con *mockController) DeleteGroup(name string) error                      { return nil }
func (con *mockController) AddResourceControl(name string, opts JobOpts) error { return nil }

func TestJobWorker_Can_Start_A_Job_And_Tail_Logs(t *testing.T) {
	WORKER_UID = -1
	WORKER_GUID = -1
	n := 5
	echo := "hello"
	cmd := "bash"
	args := []string{"-c", fmt.Sprintf("for run in {1..%d}; do echo ${run}: %s; sleep 0.01; done", n, echo)}
	opts := JobOpts{100, 100, 50 * CgroupMB}

	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	// Check the status and it's running
	status := job.Status()
	if !status.Running {
		t.Error("expected job to be running and it isn't : ", err)
		return
	}

	// Read all the job logs and assert the output of each line
	reader, err := job.Output(DontFollowLogs)
	if err != nil {
		t.Error("could not get reader for job's output")
		return
	}
	scanner := bufio.NewScanner(reader)
	logs := []string{}
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}

	for i, log := range logs {
		expected := fmt.Sprintf("%d: %s", i+1, echo)
		if log != expected {
			t.Errorf("log output was not as expected, actual %s expected %s", log, expected)
		}
	}
}

func TestJobWorker_Can_Stop_Long_Running_Job(t *testing.T) {
	WORKER_UID = -1
	WORKER_GUID = -1
	cmd := "bash"
	args := []string{"-c", "while true; do sleep 2; done"}
	opts := JobOpts{100, 100, 50 * CgroupMB}

	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	// Check the status and it's running
	status := job.Status()
	if !status.Running {
		t.Error("expected job to be running and it isn't : ", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Stop the job
	if err = job.Stop(ctx); err != nil {
		t.Errorf("expected to be able to stop the job, error : %v", err)
	}
}

func TestJobWorker_Check_Status_After_Job_Completes(t *testing.T) {
	WORKER_UID = -1
	WORKER_GUID = -1
	cmd := "bash"
	args := []string{"-c", "echo hello world"}
	opts := JobOpts{100, 100, 50 * CgroupMB}

	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status := job.Status()
	if status.Running {
		t.Error("expected job not to be running and it is")
		return
	}
	if status.ExitCode != 0 {
		t.Errorf("expected exit code to be 0 and was %d", status.ExitCode)
	}
}

func TestJobWorker_Check_Exit_Code_Is_Propagated(t *testing.T) {
	WORKER_UID = -1
	WORKER_GUID = -1
	cmd := "bash"
	args := []string{"-c", "exit 4"}
	opts := JobOpts{100, 100, 50 * CgroupMB}

	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status := job.Status()
	if status.Running {
		t.Error("expected job not to be running and it is")
		return
	}
	// Assert exit code 4 was set in status
	if status.ExitCode != 4 {
		t.Errorf("expected exit code to be 4 and was %d", status.ExitCode)
	}
}
