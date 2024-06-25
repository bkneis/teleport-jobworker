package jobworker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type CgroupByte int64

func (b CgroupByte) String() string {
	return fmt.Sprintf("%d", b)
}

// Base 2 byte units to parse / set JobOpts.MemLimit
const (
	CgroupKB CgroupByte = 1024
	CgroupMB            = CgroupKB * 1024
	CgroupGB            = CgroupMB * 1024
)

// Job maintains the exec.Cmd struct (containing the underlying os.Process) and the "owner" for authz
type Job struct {
	sync.RWMutex
	ID      string
	running bool
	cmd     *exec.Cmd
	con     ResourceController
}

// JobOpts wraps the options that can be passed to cgroups for the job
// details at https://facebookmicrosites.github.io/cgroup2/docs/overview
type JobOpts struct {
	CpuWeight int32      // `cpu.weight`
	IOLatency int32      // `io.latency`
	MemLimit  CgroupByte // `mem.high`
}

// JobStatus is an amalgamation of the useful status information available from the exec.Cmd struct of the job and it's underlying os.Process
type JobStatus struct {
	ID       string
	PID      int
	Running  bool
	ExitCode int
}

func (status JobStatus) String() string {
	return fmt.Sprintf(`
		Job Status
		ID	 %s
		PID	 %d
		Running	 %t
		ExitCode %d
	`, status.ID, status.PID, status.Running, status.ExitCode)
}

// ResourceController defines the interface for implementing resource control of new processes
// In cgroups this will be creating, editing and deleting files in /sys/fs/cgroup
type ResourceController interface {
	AddProcess(string, *exec.Cmd) error
	CreateGroup(string) error
	DeleteGroup(string) error
	AddResourceControl(string, JobOpts) error
}

// NewOpts returns a configured JobOpts based on arguments
func NewOpts(weight, ioLatency int32, memLimit CgroupByte) JobOpts {
	return JobOpts{
		CpuWeight: weight,
		IOLatency: ioLatency,
		MemLimit:  memLimit,
	}
}

// NewJob initialises a Job
func NewJob(id string, cmd *exec.Cmd, con ResourceController) *Job {
	return &Job{ID: id, running: true, cmd: cmd, con: con}
}

// Start calls start using the default ResourceController Cgroup
func Start(opts JobOpts, cmd string, args ...string) (j *Job, err error) {
	return start(&Cgroup{"/sys/fs/cgroup"}, opts, cmd, args...)
}

// start creates a job's cgroup, add the resource controls from opts. It also creates a log file for the cgroup and
// set's it to the exec.Cmd STDOUT and STDERR. Then it wraps the command executed for the job to add the PID to the cgroup
// before running the actual job's command
func start(con ResourceController, opts JobOpts, cmd string, args ...string) (j *Job, err error) {
	id := uuid.New().String()

	// Create the job
	j = NewJob(id, exec.Command(cmd, args...), con)

	// Create the cgroup and configure the controllers
	if err = j.con.CreateGroup(id); err != nil {
		return nil, err
	}

	// Update cgroup controllers to add resource control to process
	if err = j.con.AddResourceControl(id, opts); err != nil {
		return nil, err
	}

	// Add job's process to cgroup
	if err = j.con.AddProcess(id, j.cmd); err != nil {
		return nil, err
	}
	defer syscall.Close(j.cmd.SysProcAttr.CgroupFD)

	// Don't inherit environment from parent
	j.cmd.Env = []string{}

	// Pipe STDOUT and STDERR to a log file
	f, err := os.OpenFile(logPath(id), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	j.cmd.Stdout = f
	j.cmd.Stderr = f

	// Start the job
	err = j.cmd.Start()
	if err != nil {
		return nil, err
	}

	// Run go routine to handle the blocking call exec.Cmd.Wait() and update the running flag to indicate the job has complete
	go func(runningJob *Job, logFile *os.File) {
		runningJob.cmd.Wait()
		runningJob.Lock()
		runningJob.running = false
		runningJob.Unlock()
		logFile.Close()
	}(j, f)

	return j, nil
}

// Stop request a job's termination using SIGTERM and deletes it's cgroup
func (job *Job) Stop() error {
	// Request the process to terminate
	if err := job.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	// Poll the process with a signal 0 to check if it is running
	running := true
	killTime := time.Now().Add(time.Minute)
	for killTime.Unix() > time.Now().Unix() {
		if err := job.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			running = false
			break
		}
		time.Sleep(time.Second)
	}
	// After 60 second grace period kill the process with SIGKILL if still running
	if running {
		if err := job.cmd.Process.Kill(); err != nil {
			return err
		}
	}
	// Clean up job logs
	err := os.Remove(logPath(job.ID))
	if err != nil {
		return err
	}
	// Delete job's cgroup
	return job.con.DeleteGroup(job.ID)
}

// Status generates a JobStatus with information from the job and it's underlying os.Process & os.ProcessState
func (job *Job) Status() (JobStatus, error) {
	// Get PID and possible exit code from Process and ProcessState, assume running if ProcessState is nil
	pid := 0
	if job.cmd.Process != nil {
		pid = job.cmd.Process.Pid
	}
	// Check if running flag has been set after blocking Wait call on job.cmd
	job.RLock()
	running := job.running
	job.RUnlock()
	exitCode := 0
	if !running {
		exitCode = job.cmd.ProcessState.ExitCode()
	}
	return JobStatus{
		ID:       job.ID,
		PID:      pid,
		Running:  running,
		ExitCode: exitCode,
	}, nil
}

// Output returns a wrapped io.ReadCloser that "tails" the job's log file by polling for updates in Read()
func (job *Job) Output() (reader io.ReadCloser, err error) {
	return newTailReader(logPath(job.ID), 500*time.Millisecond) // todo in production this would need to be parameterized or an env var
}

// logPath returns the file path for a job's log
// todo in production this would need to be in a folder with resource isolation using the job's PID
func logPath(id string) string {
	return fmt.Sprintf("/tmp/%s.log", id)
}
