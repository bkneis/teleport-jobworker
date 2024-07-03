package jobworker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// cgroup v2 interface files for supported controllers
const (
	cpuWeightFile = "cpu.weight"
	memHighFile   = "memory.high"
	ioWeightFile  = "io.weight"
)

// Cgroup implements ResourceController and provides a minimal interface for the host's cgroup
type Cgroup struct {
	rootPath string
}

// AddProcess mutates the given cmd to instruct GO to add the PID of the started process to a given cgroup
func (cg *Cgroup) AddProcess(name string, cmd *exec.Cmd) error {
	// Add job's process to cgroup
	f, err := syscall.Open(cg.groupPath(name), 0x200000, 0)
	if err != nil {
		return err
	}
	// This is where clone args and namespaces for user, PID and fs can be set
	cmd.SysProcAttr = &syscall.SysProcAttr{
		UseCgroupFD: true,
		CgroupFD:    f,
	}
	return nil
}

// CreateGroup creates a directory in the cgroup root path to signal cgroup to create a group
// TODO in production we could check here the cgroup was created correctly, such as checking cgroup.controllers file for supported controllers
func (cg *Cgroup) CreateGroup(name string) (err error) {
	return os.Mkdir(cg.groupPath(name), 0755)
}

// DeleteGroup deletes a cgroup's directory signalling cgroup to delete the group
// TODO in production before deleting a group we could check cgroup.events to ensure no processes are still running in thr cgroup
func (cg *Cgroup) DeleteGroup(name string) error {
	return os.RemoveAll(cg.groupPath(name))
}

// updateController sets the content of the controller interface file for a
// given resource controller within a CGroup (e.g. "memory.high", etc.)
func (cg *Cgroup) updateController(name string, file, val string) error {
	return os.WriteFile(filepath.Join(cg.groupPath(name), file), []byte(val), 0644)
}

// AddResourceControl updates the resource control interface file for a given cgroup using JobOpts. The
// three currently supported are CPU, memory and IO
func (cg *Cgroup) AddResourceControl(name string, opts JobOpts) (err error) {
	if err = cg.updateController(name, cpuWeightFile, fmt.Sprintf("%d", opts.CPUWeight)); err != nil {
		return err
	}
	if err = cg.updateController(name, memHighFile, fmt.Sprintf("%d", opts.MemLimit)); err != nil {
		return err
	}
	return cg.updateController(name, ioWeightFile, fmt.Sprintf("%d", opts.IOWeight))
}

// groupPath returns a given cgroup's directory path identified by name
func (cg *Cgroup) groupPath(name string) string {
	return filepath.Join(cg.rootPath, name)
}
