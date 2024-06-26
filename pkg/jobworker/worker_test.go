package jobworker

import (
	"bufio"
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

func TestJobWorker_Can_Start_A_Job_And_Tail_Logs_Then_Stop_It(t *testing.T) {
	n := 5
	echo := "hello"
	cmd := "bash"
	args := []string{"-c", fmt.Sprintf("for run in {1..%d}; do echo %s; sleep 0.1; done", n, echo)}
	opts := NewOpts(100, 100, 50)

	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	// Check the status and it's running
	status, err := job.Status()
	if err != nil {
		t.Error("failed to get status of job: ", err)
		return
	}

	if !status.Running {
		t.Error("expected job to be running and it isn't : ", err)
		return
	}

	// Assert output
	reader, err := job.Output()
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
	if err = job.Stop(); err != nil {
		t.Error("failed to stop job : ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status, err = job.Status()
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
	cmd := "bash"
	args := []string{"-c", "echo hello world"}
	opts := NewOpts(100, 100, 50)

	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status, err := job.Status()
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
	cmd := "bash"
	args := []string{"-c", "exit 4"}
	opts := NewOpts(100, 100, 50)

	// Run the job
	job, err := StartWithController(&mockController{}, opts, cmd, args...)
	if err != nil {
		t.Error("failed to start job: ", err)
		return
	}

	time.Sleep(50 * time.Millisecond)

	// Assert the job is not running
	status, err := job.Status()
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
