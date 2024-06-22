package jobworker

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"

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
	CpuWeight int // `cpu.weight`
	MemLimit  int // `mem.high`
	IOLatency int // `io.latency`
}

// JobStatus is an amalgamation of the useful status information available from the exec.Cmd struct of the job and it's underlying os.Process
type JobStatus struct {
	ID       string
	Owner    string
	PID      int
	Running  bool
	ExitCode int
}

func (status JobStatus) String() string {
	return fmt.Sprintf(`
		Job Status
		ID	 %s
		Owner	 %s
		PID	 %d
		Running	 %t
		ExitCode %d
	`, status.ID, status.Owner, status.PID, status.Running, status.ExitCode)
}

// JobNotFound is an error returned when the job ID cannot be found
type JobNotFound struct {
	id string
}

func (err *JobNotFound) Error() string {
	return fmt.Sprintf("could not find job with id %s", err.id)
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
		con:  &Cgroup{"/sys/fs/cgroup"},
	}
}

// NewOpts returns a configured JobOpts based on arguments
func NewOpts(weight, memLimit, ioLatency int) JobOpts {
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
	jobCmd := "bash"
	cgroup := fmt.Sprintf("/sys/fs/cgroup/%s/cgroup.procs", id)
	testCmd := fmt.Sprintf("echo $$ >> %s; %s", cgroup, cmd)
	for _, arg := range args {
		testCmd += fmt.Sprintf(" %s", arg)
	}
	args = []string{"-c", testCmd}
	// log.Printf("executing cmd %s %v", jobCmd, args)

	// Create the job
	job := Job{owner, exec.Command(jobCmd, args...)}

	// Create the cgroup and configure the controllers
	if err = worker.con.CreateGroup(id); err != nil {
		log.Print("failed to create group")
		return "", err
	}

	// todo do we need to wait for signal or sleep?
	// Update cgroup controllers to add resource control to process
	if err = worker.con.AddResourceControl(id, opts); err != nil {
		log.Print("failed to add resource control")
		return "", err
	}

	// Don't inherit environment from parent
	job.cmd.Env = []string{}
	// todo possible use chroot for working directory

	// Pipe STDOUT and STDERR to a log file
	log, err := os.Create(fmt.Sprintf("/tmp/%s.log", id)) // todo need to use file permissions
	job.cmd.Stdout = log
	job.cmd.Stderr = log

	// Add it to our in memory database of jobs
	worker.Lock()
	worker.jobs[id] = &job
	worker.Unlock()

	// Start the job
	return id, job.cmd.Start()
}

// Stop request job's termination and remove it's cgroup
func (worker *JobWorker) Stop(id string) error {
	worker.Lock()
	defer worker.Unlock()
	if err := worker.jobs[id].cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	return worker.con.DeleteGroup(id)
}

// Status generates a JobStatus with information from the job and it's underlying os.Process & os.ProcessState
func (worker *JobWorker) Status(id string) (JobStatus, error) {
	worker.RLock()
	defer worker.RUnlock()
	// Check the job exists
	job, ok := worker.jobs[id]
	if !ok {
		return JobStatus{}, &JobNotFound{id}
	}
	// Get PID and possibly exit code from Process and ProcessState, assume running if ProcessState is nil
	pid := 0
	exitCode := 0
	running := true
	if job.cmd.Process != nil {
		pid = job.cmd.Process.Pid
	}
	if job.cmd.ProcessState != nil {
		running = job.cmd.ProcessState.Exited()
		exitCode = job.cmd.ProcessState.ExitCode()
	}
	return JobStatus{
		ID:       id,
		Owner:    job.owner,
		PID:      pid,
		Running:  running,
		ExitCode: exitCode,
	}, nil
}

// Output returns a wrapped io.ReadCloser that "tails" the job's log file by polling for updates in Read()
func (worker *JobWorker) Output(id string) (reader io.ReadCloser, err error) {
	return newTailReader(fmt.Sprintf("/tmp/%s.log", id))
}
