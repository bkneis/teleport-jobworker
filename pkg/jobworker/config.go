package jobworker

import "time"

// In production I would extrapolate these from the environment, possible with something like https://github.com/kelseyhightower/envconfig
var (
	TAIL_POLL_INTERVAL = 500 * time.Millisecond
	STOP_GRACE_PERIOD  = time.Minute
	STOP_POLL_INTERVAL = time.Second
)
