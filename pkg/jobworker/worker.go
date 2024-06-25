package jobworker

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// JobsList stores a map of the Job's containing the linux process key'd by job id (in memory DB)
type JobsList map[string]*Job // todo maybe make this private and job or return it after Start

// Job maintains the exec.Cmd struct (containing the underlying os.Process) and the "owner" for authz
type Job struct {
	sync.RWMutex
	running bool
	owner   string
	cmd     *exec.Cmd
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
type ErrNotFound struct {
	id string
}

func (err *ErrNotFound) Error() string {
	return fmt.Sprintf("could not find job with id %s", err.id)
}

// ResourceController defines the interface for implementing resource control of new processes
// In cgroups this will be creating, editing and deleting files in /sys/fs/cgroup
type ResourceController interface {
	AddProcess(string, *exec.Cmd) error
	CreateGroup(string) error
	DeleteGroup(string) error
	AddResourceControl(string, JobOpts) error
	ProcsPath(string) string
}

// JobWorker implements Worker and will be initiated once by the binary starting the gRPC server
// todo use logger member so disabling logs or debug level can be set
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
	// jobCmd := "bash"
	// testCmd := fmt.Sprintf("echo $$ >> %s; %s", worker.con.ProcsPath(id), cmd)
	// // todo use string.Builder??
	// for _, arg := range args {
	// 	testCmd += fmt.Sprintf(" %s", arg)
	// }
	// args = []string{"-c", testCmd}

	// Create the job
	job := Job{running: true, owner: owner, cmd: exec.Command(cmd, args...)}

	// Create the cgroup and configure the controllers
	if err = worker.con.CreateGroup(id); err != nil {
		log.Print("failed to create group")
		return "", err
	}

	// Update cgroup controllers to add resource control to process
	if err = worker.con.AddResourceControl(id, opts); err != nil {
		log.Print("failed to add resource control")
		return "", err
	}

	// // Add job's process to cgroup
	// f, err := syscall.Open(fmt.Sprintf("/sys/fs/cgroup/%s", id), 0x200000, 0)
	// if err != nil {
	// 	log.Print("could not open procs file")
	// 	return "", err
	// }
	// defer syscall.Close(f)

	// // This is where clone args and namespaces for user, PID and fs can be set
	// job.cmd.SysProcAttr = &syscall.SysProcAttr{
	// 	UseCgroupFD: true,
	// 	CgroupFD:    f,
	// }
	if err = worker.con.AddProcess(id, job.cmd); err != nil {
		log.Print("failed to add process to cgroup")
		return "", err
	}
	defer syscall.Close(job.cmd.SysProcAttr.CgroupFD)

	// Don't inherit environment from parent
	job.cmd.Env = []string{}
	// todo should we use chroot for working directory??

	// Pipe STDOUT and STDERR to a log file
	lf, err := os.OpenFile(logPath(id), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Print("could not create log file for job: ", err)
		return "", err
	}
	job.cmd.Stdout = lf
	job.cmd.Stderr = lf

	// Start the job and add it to our in memory database of jobs
	worker.Lock()
	worker.jobs[id] = &job
	err = job.cmd.Start()
	worker.Unlock()
	if err != nil {
		log.Print("could not start job ", err)
		return "", err
	}

	// Run go routine to handle the blocking call exec.Cmd.Wait() and update the running flag to indicate the job has complete
	go func(j *Job, logFile *os.File) {
		j.cmd.Wait()
		j.Lock()
		j.running = false
		j.Unlock()
		logFile.Close()
	}(&job, lf)

	return id, nil
}

// Stop request a job's termination using SIGTERM and deletes it's cgroup
func (worker *JobWorker) Stop(id string) error {
	worker.Lock()
	defer worker.Unlock()
	job, ok := worker.jobs[id]
	if !ok {
		return &ErrNotFound{id}
	}
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
	err := os.Remove(logPath(id))
	if err != nil {
		return err
	}
	// Delete job's cgroup
	return worker.con.DeleteGroup(id)
}

// Status generates a JobStatus with information from the job and it's underlying os.Process & os.ProcessState
func (worker *JobWorker) Status(id string) (JobStatus, error) {
	worker.RLock()
	defer worker.RUnlock()
	// Check the job exists
	job, ok := worker.jobs[id]
	if !ok {
		return JobStatus{}, &ErrNotFound{id}
	}
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
		ID:       id,
		Owner:    job.owner,
		PID:      pid,
		Running:  running,
		ExitCode: exitCode,
	}, nil
}

// Output returns a wrapped io.ReadCloser that "tails" the job's log file by polling for updates in Read()
// todo parameterize pollInterval
func (worker *JobWorker) Output(id string) (reader io.ReadCloser, err error) {
	return newTailReader(logPath(id), 500*time.Millisecond)
}

// logPath returns the file path for a job's log
// In production this would need to be in a folder with resource isolation using the job's PID
func logPath(id string) string {
	return fmt.Sprintf("/tmp/%s.log", id)
}
