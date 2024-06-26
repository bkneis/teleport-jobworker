package doc

import (
	"io"
	"os"
	"os/exec"
	"time"
)

type CgroupByte int64

// Base 2 byte units to parse / set JobOpts.MemLimit
const (
	CgroupKB CgroupByte = 1024
	CgroupMB            = CgroupKB * 1024
	CgroupGB            = CgroupMB * 1024
)

// Job maintains the exec.Cmd struct (containing the underlying os.Process) and is returned
// after a successful call to Start and provides an API for interacting with the job
type Job struct {
	cmd *exec.Cmd
	con ResourceController
}

func (job *Job) Stop() error                    { return nil }
func (job *Job) Status() JobStatus              { return JobStatus{} }
func (job *Job) Output() (io.ReadCloser, error) { return nil, nil }

// JobOpts wraps the options that can be passed to cgroups for the job
// details at https://facebookmicrosites.github.io/cgroup2/docs/overview
type JobOpts struct {
	CPUWeight int32      // `cpu.weight` value between [1, 1000]
	MemLimit  CgroupByte // `mem.high` bytes to throttle memory usage
	IOLatency int32      // `io.latency` using ms as the unit
}

// JobStatus is an amalgamation of the useful status information available from the exec.Cmd struct of the job and it's underlying os.Process
type JobStatus struct {
	ID       string
	PID      int
	Running  bool
	ExitCode int
}

// ResourceController defines the interface for implementing resource control of new processes
// In cgroups this will be creating, editing and deleting files in /sys/fs/cgroup
type ResourceController interface {
	AddProcess(string, *exec.Cmd) error
	CreateGroup(string) error
	DeleteGroup(string) error
	AddResourceControl(JobOpts) error
}

// Start creates a cgroup, implements resource control and executes a Command with a go routine performing Wait to process the ExitCode
func Start(opts JobOpts, name string, args ...string) (job *Job, err error) {
	// setup exec.Cmd, call relevant resource controller functions etc.
	return job, nil
}

// Example io.ReadCloser wrapper to tail logs from the os.File and sleep pollInterval before reading again for updates
type tailReader struct {
	io.ReadCloser
	pollInterval time.Duration
}

func (t tailReader) Read(b []byte) (int, error) {
	for {
		n, err := t.ReadCloser.Read(b)
		if n > 0 {
			return n, nil
		} else if err != io.EOF {
			return n, err
		}
		time.Sleep(t.pollInterval)
	}
}

func newTailReader(pollInterval time.Duration, fileName string) (tailReader, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return tailReader{}, err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return tailReader{}, err
	}
	return tailReader{f, pollInterval}, nil
}
