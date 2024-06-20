package doc

import (
	"io"
	"os/exec"
)

// JobsList stores a map of the Job's containing the linux process key'd by job id (in memory DB)
type JobsList map[string]*Job

// Job maintains the exec.Cmd struct (containing the underlying os.Process) and the "owner" for authz
type Job struct {
	owner string
	cmd   *exec.Cmd
}

// JobOpts wraps the options that can be passed to cgroups for the job
// details at https://facebookmicrosites.github.io/cgroup2/docs/overview
type JobOpts struct {
	CpuWeight int    // `cpu.weight`
	MemLimit  string // `mem.high`
	IOLatency string // `io.latency`
}

// JobStatus is an amalgamation of the useful status information available from the exec.Cmd struct of the job and it's underlying os.Process
type JobStatus struct {
	ID          string
	Owner       string
	PID         int8
	Running     bool
	ExitCode    uint8
	ReturnError error
}

// Worker defines the libraries API on how to start / stop / query status and get output of a job
type Worker interface {
	Start(owner, cmd string, opts JobOpts) (id string, err error)
	Stop(id string) error
	Status(id string) (JobStatus, error)
	Output(id string) (io.Reader, error)
}

// ResourceController defines the interface for implementing resource control of new processes
// In cgroups this will be creating, editing and deleting files in /sys/fs/cgroup
type ResourceController interface {
	AddProcess(int) error
	CreateGroup(string) error
	DeleteGroup(string) error
	AddResourceControl(JobOpts) error
}

// JobWorker implements Worker and will be initiated once by the binary starting the gRPC server
type JobWorker struct {
	jobs JobsList
	con  ResourceController
}
