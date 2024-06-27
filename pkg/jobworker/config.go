package jobworker

import "time"

// TODO In production I would extrapolate these from the environment, possible with something like https://github.com/kelseyhightower/envconfig
var (
	TAIL_POLL_INTERVAL = 500 * time.Millisecond
	STOP_GRACE_PERIOD  = time.Minute
	STOP_POLL_INTERVAL = time.Second
	RPC_CLIENT_TIMEOUT = 10 * time.Second
	RPC_STREAM_TIMEOUT = 10 * time.Minute
	WORKER_UID         = 1000
	WORKER_GID         = 1000
)
