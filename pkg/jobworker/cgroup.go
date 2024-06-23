package jobworker

import (
	"fmt"
	"os"
)

// Cgroup implements ResourceController and provides a minimal interface for the host's cgroup
type Cgroup struct {
	rootPath string
}

// AddProcess writes a given PID to a cgroup of the same name
// func (cg *Cgroup) AddProcess(name string, pid int) error {
// 	// Open the cgroups process list
// 	f, err := os.OpenFile(fmt.Sprintf("%s/%s/cgroup.procs", cg.rootPath, name), os.O_APPEND|os.O_WRONLY, 0644)
// 	if err != nil {
// 		return err
// 	}
// 	// Write job's PID to cgroup
// 	if _, err = f.Write([]byte(fmt.Sprintf("%d\n", pid))); err != nil {
// 		return err
// 	}
// 	return f.Close()
// }

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

// RootPath todo not sure on this
func (cg *Cgroup) RootPath() string {
	return cg.rootPath
}

// groupPath returns a given cgroup's directory path identified by name
func (cg *Cgroup) groupPath(name string) string {
	return fmt.Sprintf("%s/%s", cg.rootPath, name)
}
