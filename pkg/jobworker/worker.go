package jobworker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
)

// CgroupByte is used as a Byte to parse JobOpts.MemLimit cgroup value
type CgroupByte int64

func (b CgroupByte) String() string {
	return fmt.Sprintf("%d", b)
}

func parseCgroupValue(value, unit string) (CgroupByte, error) {
	parts := strings.Split(value, unit)
	if len(parts) < 2 {
		return 0, fmt.Errorf("cgroup value not valid")
	}
	v, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not convert cgroup value to int: %w", err)
	}
	return CgroupByte(v), nil
}

// ParseCgroupByte returns a CgroupByte value based on a string
// TODO test
func ParseCgroupByte(value string) (n CgroupByte, err error) {
	if strings.Contains(value, "K") {
		if n, err = parseCgroupValue(value, "K"); err != nil {
			return 0, err
		}
		return n * CgroupKB, nil
	} else if strings.Contains(value, "M") {
		if n, err = parseCgroupValue(value, "M"); err != nil {
			return 0, err
		}
		return n * CgroupMB, nil
	} else if strings.Contains(value, "G") {
		if n, err = parseCgroupValue(value, "G"); err != nil {
			return 0, err
		}
		return n * CgroupGB, nil
	}
	return 50 * CgroupMB, nil
}

const (
	CgroupKB CgroupByte = 1024
	CgroupMB            = CgroupKB * 1024
	CgroupGB            = CgroupMB * 1024
)

// OutputMode determines if a call to Output should follow the logs or not, similar to tail -f
type OutputMode int

const (
	DontFollowLogs OutputMode = 0
	FollowLogs     OutputMode = 1
)

// Job maintains the exec.Cmd struct (containing the underlying os.Process) and the "owner" for authz
type Job struct {
	sync.RWMutex
	ID      string
	running bool
	done    chan bool
	pgid    int
	cmd     *exec.Cmd
	readers []io.ReadCloser
	con     ResourceController
}

// JobOpts wraps the options that can be passed to cgroups for the job
// details at https://facebookmicrosites.github.io/cgroup2/docs/overview
type JobOpts struct {
	CPUWeight int32      // `cpu.weight`
	IOWeight  int32      // `io.weight`
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
	return fmt.Sprintf(`Job Status
	ID	%s
	PID	%d
	Running	%t
	ExitCode %d`, status.ID, status.PID, status.Running, status.ExitCode)
}

// ResourceController defines the interface for implementing resource control of new processes
// In cgroups this will be creating, editing and deleting files in /sys/fs/cgroup
type ResourceController interface {
	AddProcess(string, *exec.Cmd) error
	CreateGroup(string) error
	DeleteGroup(string) error
	AddResourceControl(string, JobOpts) error
}

// NewJob initialises a Job
func NewJob(id string, cmd *exec.Cmd, con ResourceController) *Job {
	return &Job{ID: id, running: true, cmd: cmd, con: con, done: make(chan bool, 1), readers: []io.ReadCloser{}}
}

// Start calls start using the default ResourceController Cgroup
func Start(opts JobOpts, cmd string, args ...string) (j *Job, err error) {
	return StartWithController(&Cgroup{"/sys/fs/cgroup"}, opts, cmd, args...)
}

// StartWithController creates a job's cgroup, adds the resource controls from opts, creates a log file for the cgroup and
// set's it to the exec.Cmd STDOUT and STDERR. Finally we Start the exec.Cmd and start a go routine to handle the blocking
// call to Wait so that we can update the job's running flag.
func StartWithController(con ResourceController, opts JobOpts, cmd string, args ...string) (j *Job, err error) {
	// Create the job
	j = NewJob(uuid.New().String(), exec.Command(cmd, args...), con)
	// Create the cgroup and configure the controllers
	if err = j.con.CreateGroup(j.ID); err != nil {
		return nil, fmt.Errorf("failed to create cgroup: %w", err)
	}
	// Update cgroup controllers to add resource control to process
	if err = j.con.AddResourceControl(j.ID, opts); err != nil {
		return nil, fmt.Errorf("failed to add resource control: %w", err)
	}
	// Add job's process to cgroup
	if err = j.con.AddProcess(j.ID, j.cmd); err != nil {
		return nil, fmt.Errorf("failed to add PID to cgroup: %w", err)
	}
	defer syscall.Close(j.cmd.SysProcAttr.CgroupFD)
	// Don't inherit environment from parent
	j.cmd.Env = []string{}
	// Pipe STDOUT and STDERR to a log file
	f, err := os.OpenFile(logPath(j.ID), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open job's log file: %w", err)
	}
	j.cmd.Stdout = f
	j.cmd.Stderr = f
	// Run the command as a given user as not to escalate privilege, since the executing user must also manage cgroups
	if WORKER_UID != -1 && WORKER_GID != -1 {
		j.cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(WORKER_UID), Gid: uint32(WORKER_GID)}
	}
	j.cmd.SysProcAttr.Setpgid = true
	// Start the job
	err = j.cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start job's exec.Cmd: %w", err)
	}
	// Assign the process group ID to the job so that we have a reference to signal child processes in Stop if the command quits
	j.pgid, err = syscall.Getpgid(j.cmd.Process.Pid)
	if err != nil {
		fmt.Printf("error getting pgid: %v", err)
		return nil, err
	}
	// Run go routine to handle the blocking call exec.Cmd.Wait() and update the running flag to indicate the job has complete
	go func(runningJob *Job, logFile *os.File) {
		runningJob.cmd.Wait()
		runningJob.setRunning(false)
		runningJob.done <- true
		logFile.Close()
		// Close all of the readers reading the logs
		for _, r := range runningJob.readers {
			r.Close()
		}
	}(j, f)
	return j, nil
}

// Stop request a job's termination using SIGTERM and deletes it's cgroup. If the SIGTERM is ignored, we send
// a SIGKILL after STOP_GRACE_PERIOD.
func (job *Job) Stop(ctx context.Context) error {
	// Regardless of signalling errors, ensure we clean up the job's log file and cgroup
	defer func() {
		os.Remove(logPath(job.ID))
		job.con.DeleteGroup(job.ID)
	}()
	// Send SIGTERM to process group
	if err := syscall.Kill(-job.pgid, syscall.SIGTERM); err != nil {
		return err
	}
	// Set up timeout
	killCtx, cancel := context.WithTimeout(context.Background(), STOP_GRACE_PERIOD)
	defer cancel()
	// If job is running potentially wait on either done channel or SIGKILL after timeout
	if job.isRunning() {
		select {
		case <-job.done:
			break
		case <-killCtx.Done():
			if err := syscall.Kill(-job.pgid, syscall.SIGKILL); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Status generates a JobStatus with information from the job and it's underlying os.Process & os.ProcessState
func (job *Job) Status() JobStatus {
	// Get PID and possible exit code from Process and ProcessState, assume running if ProcessState is nil
	pid := 0
	if job.cmd.Process != nil {
		pid = job.cmd.Process.Pid
	}
	// Check if running flag has been set after blocking Wait call on job.cmd
	running := job.isRunning()
	exitCode := 0
	if !running {
		exitCode = job.cmd.ProcessState.ExitCode()
	}
	return JobStatus{
		ID:       job.ID,
		PID:      pid,
		Running:  running,
		ExitCode: exitCode,
	}
}

// Output returns a wrapped io.ReadCloser that "tails" the job's log file
// If follow=true, the Read will block and poll for updates to the file. It is then the responsibility
// of the caller to ensure .Close is called to prevent the Read blocking indefinitely.
// If follow=false, upon Read'ing the entire file an io.EOF will be returned
func (job *Job) Output(mode OutputMode) (reader io.ReadCloser, err error) {
	reader, err = newTailReader(logPath(job.ID), TAIL_POLL_INTERVAL, mode)
	if err != nil {
		return nil, err
	}
	job.Lock()
	job.readers = append(job.readers, reader)
	job.Unlock()
	return reader, nil
}

func (job *Job) setRunning(running bool) {
	job.Lock()
	defer job.Unlock()
	job.running = running
}

func (job *Job) isRunning() bool {
	job.RLock()
	defer job.RUnlock()
	return job.running
}

// logPath returns the file path for a job's log
// TODO in production this would need to be in a folder with resource isolation using the job's PID
func logPath(id string) string {
	return filepath.Join("/tmp/", fmt.Sprintf("%s.log", id))
}
