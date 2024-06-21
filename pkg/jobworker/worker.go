package jobworker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/google/uuid"
)

// JobsList stores a map of the Job's containing the linux process key'd by job id (in memory DB)
type JobsList map[string]*Job // todo maybe make this private and job

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
	ID       string
	Owner    string
	PID      int
	Running  bool
	ExitCode int
}

// Worker defines the libraries API on how to start / stop / query status and get output of a job
type Worker interface {
	Start(opts JobOpts, owner, cmd string, args ...string) (id string, err error)
	Stop(id string) error
	Status(id string) (JobStatus, error)
	Output(id string) (io.Reader, error)
}

// ResourceController defines the interface for implementing resource control of new processes
// In cgroups this will be creating, editing and deleting files in /sys/fs/cgroup
type ResourceController interface {
	// AddProcess(string, int) error
	CreateGroup(string) error
	DeleteGroup(string) error
	AddResourceControl(string, JobOpts) error
}

// JobWorker implements Worker and will be initiated once by the binary starting the gRPC server
type JobWorker struct {
	sync.RWMutex
	jobs JobsList
	con  ResourceController
}

// New returns an initialized JobWorker with the cgroup resource controller
func New() *JobWorker {
	return &JobWorker{
		jobs: JobsList{},
		con:  &Cgroup{"/sys/fs/cgroup/"},
	}
}

// NewOpts returns a configured JobOpts based on arguments
func NewOpts(weight int, memLimit, ioLatency string) JobOpts {
	return JobOpts{
		CpuWeight: weight,
		MemLimit:  memLimit,
		IOLatency: ioLatency,
	}
}

// Start creates a job's cgroup, add the resource controls from opts. It also creates a log file for the cgroup and
// set's it to the exec.Cmd STDOUT and STDERR. Then it wraps the command executed for the job to add the PID to the cgroup
// before running the actual job's command
func (worker *JobWorker) Start(opts JobOpts, owner, cmd string, args ...string) (id string, err error) {
	id = uuid.New().String()
	// Prefix the cmd and args with a command to add the PID to the cgroup
	// todo test
	jobCmd := fmt.Sprintf("echo $$ > /sys/fs/cgroup/%s/cgroup.procs; %s", id, cmd)
	// Create the job
	job := Job{owner, exec.Command(jobCmd, args...)}

	// Create the cgroup and configure the controllers
	if err = worker.con.CreateGroup(id); err != nil {
		return "", err
	}
	if err = worker.con.AddResourceControl(id, opts); err != nil {
		return "", err
	}

	// Don't inherit environment from parent
	job.cmd.Env = []string{}
	// todo need to check chroot of working directory

	// Pipe STDOUT and STDERR to a log file
	log, err := os.Create(fmt.Sprintf("/tmp/%s.log", id)) // todo need to use file permissions
	job.cmd.Stdout = log
	job.cmd.Stderr = log

	// Add it to our in memory database of jobs
	worker.Lock()
	defer worker.Unlock()
	worker.jobs[id] = &job
	// Start the job
	return id, job.cmd.Start()
}

// Stop kills the job's process and removes it's cgroup
func (worker *JobWorker) Stop(id string) error {
	worker.Lock()
	if err := worker.jobs[id].cmd.Process.Kill(); err != nil {
		return err
	}
	worker.Unlock() // not defer'ing as not to wait for the cgroup file to be deleted
	return worker.con.DeleteGroup(id)
}

// Status generates a JobStatus with information from the job and it's underlying os.Process & os.ProcessState
func (worker *JobWorker) Status(id string) JobStatus {
	worker.RLock()
	defer worker.RUnlock()
	return JobStatus{
		ID:       id,
		Owner:    worker.jobs[id].owner,
		PID:      worker.jobs[id].cmd.Process.Pid,
		Running:  worker.jobs[id].cmd.ProcessState.Exited(),
		ExitCode: worker.jobs[id].cmd.ProcessState.ExitCode(),
	}
}

// Output executes tail -f and pipes the STDOUT to an io.Reader that it returned to the caller
func (worker *JobWorker) Output(id string) (reader io.Reader, err error) {
	// Tail the job's log and follow
	cmd := exec.Command("tail", "-f", fmt.Sprintf("/tmp/%s.log", id))
	// Get an io.Reader to the STDOUT
	if reader, err = cmd.StdoutPipe(); err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	return reader, nil
}
