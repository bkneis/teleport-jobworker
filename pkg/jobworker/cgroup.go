package jobworker

import (
	"fmt"
	"os"
)

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
func (cg *Cgroup) CreateGroup(name string) error {
	return os.Mkdir(fmt.Sprintf("%s/%s", cg.rootPath, name), 0755)
}

func (cg *Cgroup) DeleteGroup(name string) error {
	return os.RemoveAll(fmt.Sprintf("%s/%s", cg.rootPath, name))
}

// updateController overrides a given cgroup controller's interface file with a value
func (cg *Cgroup) updateController(name string, file, val string) error {
	controller := fmt.Sprintf("%s/%s/%s", cg.rootPath, name, file)
	return os.WriteFile(controller, []byte(val), 0644)
}

func (cg *Cgroup) AddResourceControl(name string, opts JobOpts) (err error) {
	// Update each of the three supported cgroup controllers with the job options
	if err = cg.updateController(name, "cpu.weight", fmt.Sprintf("%d", opts.CpuWeight)); err != nil {
		return err
	}
	if err = cg.updateController(name, "memory.high", fmt.Sprintf("%dM", opts.MemLimit)); err != nil {
		return err
	}
	return cg.updateController(name, "io.weight", fmt.Sprintf("%d", opts.IOLatency)) // todo change to io.latency and use ms
}
