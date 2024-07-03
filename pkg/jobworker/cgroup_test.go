package jobworker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const testName = "TEST"

var testOpts = JobOpts{100, 50, 100 * CgroupMB}

// exists returns whether the given file or directory exists
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func TestCgroupController(t *testing.T) {
	testDir := fmt.Sprintf("/tmp/%s", testName)
	err := os.RemoveAll(testDir)
	if err != nil {
		t.Errorf("failed to clean up test dir: %v", err)
		return
	}
	cgroup := Cgroup{"/tmp/"}
	// TEST CreateGroup
	err = cgroup.CreateGroup(testName)
	if err != nil {
		t.Errorf("could not create cgroup: %v", err)
		return
	}
	// assert cgroup exists
	exist, err := exists(testDir)
	if !exist || err != nil {
		t.Error("Expected /tmp/TEST to exist to represent cgroup")
	}

	// TEST AddResourceControl
	err = cgroup.AddResourceControl(testName, testOpts)
	if err != nil {
		t.Errorf("could not add resource controls to cgroup controller: %v", err)
		return
	}
	// assert /tmp/test/cgroup controller files are updated
	// cpu
	cpuWeight, err := os.ReadFile(filepath.Join(testDir, cpuWeightFile))
	if err != nil {
		t.Errorf("could not read CPU cgroup controller: %v", err)
		return
	}
	if string(cpuWeight) != "100" {
		t.Errorf("CPU weight is incorrect: %v", err)
		return
	}
	// mem
	mem, err := os.ReadFile(filepath.Join(testDir, memHighFile))
	if err != nil {
		t.Errorf("could not read memory cgroup controller: %v", err)
		return
	}
	expectedMem := "104857600"
	if string(mem) != expectedMem {
		t.Errorf("memory high is incorrect: expected %s actual %s", expectedMem, string(mem))
		return
	}
	// io
	ioLatency, err := os.ReadFile(filepath.Join(testDir, ioWeightFile))
	if err != nil {
		t.Errorf("could not read IO cgroup controller: %v", err)
		return
	}
	if string(ioLatency) != "50" {
		t.Errorf("IO latency is incorrect: %v", err)
		return
	}
	// TEST DeleteGroup
	err = cgroup.DeleteGroup(testName)
	if err != nil {
		t.Errorf("could not delete cgroup: %v", err)
		return
	}
	// assert file is not there
	exist, err = exists(testDir)
	if exist || err != nil {
		t.Error("Expected /tmp/TEST NOT to exist to represent cgroup")
	}
}
