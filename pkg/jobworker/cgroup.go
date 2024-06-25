package jobworker

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Cgroup implements ResourceController and provides a minimal interface for the host's cgroup
// todo do we need to check cgroup.controllers have three supported controllers??
type Cgroup struct {
	rootPath string
}

// ErrControllerNotSupported is returned if the cgroup v2 controller is not supported on the host
type ErrControllerNotSupported struct {
	rootPath   string
	controller string
}

func (err ErrControllerNotSupported) Error() string {
	return fmt.Sprintf("cgroup v2 controller %s not available in %s/cgroup.subtree_control", err.controller, err.rootPath)
}

// NewCgroup returns an initialized Cgroup and checks for controller compatibility on the host
func NewCgroup(rootPath string) (cg *Cgroup, err error) {
	cg = &Cgroup{rootPath}
	var subtree []byte
	// todo is this needed?
	if subtree, err = os.ReadFile(fmt.Sprintf("%s/cgroup.subtree_control", cg.rootPath)); err != nil {
		return nil, err
	}
	if strings.Contains(string(subtree), "cpu") {
		return nil, &ErrControllerNotSupported{rootPath, "cpu"}
	}
	if strings.Contains(string(subtree), "memory") {
		return nil, &ErrControllerNotSupported{rootPath, "cpu"}
	}
	if strings.Contains(string(subtree), "io") {
		return nil, &ErrControllerNotSupported{rootPath, "cpu"}
	}
	return cg, nil
}

func (cg *Cgroup) AddProcess(name string, cmd *exec.Cmd) error {
	// Add job's process to cgroup
	f, err := syscall.Open(cg.groupPath(name), 0x200000, 0)
	if err != nil {
		log.Print("could not open procs file")
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
func (cg *Cgroup) CreateGroup(name string) (err error) {
	groupPath := cg.groupPath(name)
	if err := os.Mkdir(groupPath, 0755); err != nil {
		return err
	}
	// todo is this important? if not revert to single liner
	// check cgroup populated directory
	// _, err = os.Stat(fmt.Sprintf("%s/cgroup.controllers", groupPath))
	return err
}

// DeleteGroup deletes a cgroup's directory signalling cgroup to delete the group
// todo first maybe check cgroup.events https://docs.kernel.org/admin-guide/cgroup-v2.html#basic-operations
func (cg *Cgroup) DeleteGroup(name string) error {
	return os.RemoveAll(cg.groupPath(name))
}

// updateController overrides a given cgroup controller's interface file with a value
func (cg *Cgroup) updateController(name string, file, val string) error {
	return os.WriteFile(fmt.Sprintf("%s/%s", cg.groupPath(name), file), []byte(val), 0644)
}

// AddResourceControl updates the resource control interface file for a given cgroup using JobOpts. The
// three currently supported are CPU, memory and IO
func (cg *Cgroup) AddResourceControl(name string, opts JobOpts) (err error) {
	if err = cg.updateController(name, "cpu.weight", fmt.Sprintf("%d", opts.CpuWeight)); err != nil {
		return err
	}
	if err = cg.updateController(name, "memory.high", fmt.Sprintf("%dM", opts.MemLimit)); err != nil {
		return err
	}
	return cg.updateController(name, "io.weight", fmt.Sprintf("%d", opts.IOLatency)) // todo change to io.latency and use ms
}

// ProcsPath returns the file path to append the PID to add to a cgroup
func (cg *Cgroup) ProcsPath(name string) string {
	return fmt.Sprintf("%s/cgroup.procs", cg.groupPath(name))
}

// groupPath returns a given cgroup's directory path identified by name
func (cg *Cgroup) groupPath(name string) string {
	return fmt.Sprintf("%s/%s", cg.rootPath, name)
}
