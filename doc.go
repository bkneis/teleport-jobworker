package doc

import (
	"io"
	"os"
	"os/exec"
	"time"
)

// JobsList stores a map of the Job's containing the linux process key'd by job id (in memory DB)
type JobsList map[string]*Job

// Job maintains the exec.Cmd struct (containing the underlying os.Process) and the "owner"
type Job struct {
	owner string
	cmd   *exec.Cmd
}

// JobOpts wraps the options that can be passed to cgroups for the job
// details at https://facebookmicrosites.github.io/cgroup2/docs/overview
type JobOpts struct {
	CPUWeight int32 // `cpu.weight`
	MemLimit  int32 // `mem.high` using megabytes as the unit
	IOLatency int32 // `io.latency` using ms as the unit
}

// JobStatus is an amalgamation of the useful status information available from the exec.Cmd struct of the job and it's underlying os.Process
type JobStatus struct {
	ID       string
	Owner    string
	PID      int
	Running  bool
	ExitCode int
}

// ResourceController defines the interface for implementing resource control of new processes
// In cgroups this will be creating, editing and deleting files in /sys/fs/cgroup
type ResourceController interface {
	AddProcess(int) error
	CreateGroup(string) error
	DeleteGroup(string) error
	AddResourceControl(JobOpts) error
}

// JobWorker provides the libraries API on how to start / stop / query status and get output of a job
type JobWorker struct {
	jobs JobsList
	con  ResourceController
}

func (worker *JobWorker) Start(opts JobOpts, owner, name string, args ...string) (id string, err error) {
	return
}
func (worker *JobWorker) Stop(id string) error {
	return nil
}
func (worker *JobWorker) Status(id string) JobStatus {
	return JobStatus{}
}
func (worker *JobWorker) Output(id string) (io.ReadCloser, error) {
	return nil, nil
}

// Example io.ReadCloser wrapper to tail logs from the os.File
type tailReader struct {
	io.ReadCloser
}

func (t tailReader) Read(b []byte) (int, error) {
	for {
		n, err := t.ReadCloser.Read(b)
		if n > 0 {
			return n, nil
		} else if err != io.EOF {
			return n, err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func newTailReader(fileName string) (tailReader, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return tailReader{}, err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return tailReader{}, err
	}
	return tailReader{f}, nil
}
