package jobworker

import "testing"

const TEST_NAME = "TEST"

var TEST_OPTS = JobOpts{100, "100M", "50ms"}

func TestCgroupController(t *testing.T) {
	cgroup := Cgroup{"/tmp/"}

	err := cgroup.CreateGroup(TEST_NAME)
	if err != nil {
		t.Errorf("could not create cgroup: %v", err)
		return
	}
	// todo assert /tmp/test/cgroup exists

	err = cgroup.AddResourceControl(TEST_NAME, TEST_OPTS)
	if err != nil {
		t.Errorf("could not add resource controls to cgroup controller: %v", err)
		return
	}
	// todo assert /tmp/test/cgroup controller files are updated

	err = cgroup.DeleteGroup(TEST_NAME)
	if err != nil {
		t.Errorf("could not delete cgroup: %v", err)
		return
	}
	// assert file is not there
}
