package doc

import (
	"io"
	"os"
	"os/exec"
	"time"
)

// job maintains the exec.Cmd struct (containing the underlying os.Process) and implements Job
type job struct {
	cmd *exec.Cmd
}

func (job *job) Stop() error                    { return nil }
func (job *job) Status() JobStatus              { return JobStatus{} }
func (job *job) Output() (io.ReadCloser, error) { return nil, nil }

// Job is returned to the caller with a successful call to Start and provides an API for interacting with the job
type Job interface {
	Stop() error                    // sends a SIGTERM to the job's process then polls the process and sends SIGKILL if necessary
	Status() JobStatus              // returns information of the job's process
	Output() (io.ReadCloser, error) // returns a io.ReadCloser that tails the job's log file
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

// JobWorker provides the Start function that returns a Job based on JobOpts and a command with arguments
type JobWorker struct {
	con ResourceController
}

// New returns an initialized JobWorker
func New() *JobWorker {
	return &JobWorker{}
}

// Start creates a cgroup, implements resource control and executes a Command with a go routine performing Wait to process the ExitCode
func (worker *JobWorker) Start(opts JobOpts, name string, args ...string) (id Job, err error) {
	return &job{}, nil
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
