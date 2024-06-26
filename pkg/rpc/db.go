package rpc

import (
	"sync"

	"github.com/teleport-jobworker/pkg/jobworker"
)

// jobList is a map of Jobs key'd by their ID
type jobList map[string]*jobworker.Job

// InMemoryJobsDB is an in memory database of job's firstly key'd by owner and then job ID
// TODO in production this would be persisted in an actual DB
type InMemoryJobsDB struct {
	sync.RWMutex
	jobs map[string]jobList // list of jobLists key'd by owner
}

// Get returns a Job for an owner and job ID, returning nil if not found
func (db *InMemoryJobsDB) Get(owner, id string) *jobworker.Job {
	db.RLock()
	defer db.RUnlock()
	ownersJobs, ok := db.jobs[owner]
	if !ok {
		return nil
	}
	job, ok := ownersJobs[id]
	if !ok {
		return nil
	}
	return job
}

// Update upserts a job into the owner's list of jobs, where any existing job would be updated
func (db *InMemoryJobsDB) Update(owner string, job *jobworker.Job) {
	db.Lock()
	defer db.Unlock()
	_, ok := db.jobs[owner]
	if !ok {
		db.jobs[owner] = jobList{}
	}
	db.jobs[owner][job.ID] = job
}

// Remove deletes a job from a owner's job list
func (db *InMemoryJobsDB) Remove(owner string, id string) {
	db.Lock()
	defer db.Unlock()
	_, ok := db.jobs[owner]
	if !ok {
		db.jobs[owner] = jobList{}
	}
	delete(db.jobs[owner], id)
}
