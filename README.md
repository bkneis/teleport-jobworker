# JobWorker
jobworker is a simple golang library that executes arbitrary linux processes with options for resource control using cgroups v2.

The golang library uses the host file system to manage cgroups, where directory and files are created / delete in the cgroup root dir `/sys/fs/cgroup`.

Executing the linux command is done with `exec.Cmd` where the Cmd is wrapped in a Job struct that provides an API for managing the process.

The library assumes a 64 bit linux system with cgroups v2, no assurances are provided that the cgroups are correctly working. For instance when creating a group, a directory is created in the cgroup root directory to trigger a group creation, but the library does not perform some sanity check to ensure the cgroup was actually created.

## How to

Build the grpc client and server

`make all`

Start the server with race detector enabled

`sudo ./server_debug &`

Run commands on the client

`./worker start bash -c "while true; do echo hello; sleep 1; done"`
`./worker stop ...`
`./worker status ...`
`./worker logs ...`
`./worker -f logs ...`

Query the go runtime profiles at

`http://localhost:6060/debug/pprof/`

Run the libraries unit tests

`make test`

or with the race detector

`make race_test`

Additionally integration tests that test the mTLS, authz and management of the linux process can be run using a sudo user (since it actually runs the gRPC server)

`make integration_test`

Build the example

`make example`

or with the race detector and profiler enabled

`make example_debug`

Run concurrent example and view profile information such as number of go routines, heap allocations etc. It simulates N clients tailing the logs of one job, where N is the first argument of the binary. There should be 1 go routine for each reader and 1 for the job.

```bash
make concurrent
./concurrent 10 bash -c "while true; do echo hello; sleep 1; done"
google-chrome http://localhost:6060/debug/pprof/
```

Generate the protobuf go & grpc definitions after updating `pkg/proto/worker.proto`

`make proto`

Lint the code for static analysis

`make lint`

View test coverage

`make coverage`

Example long lived command with tailing

`./example bash -c "while true; do echo hello; sleep 2; done"`

Ctrl+C to signal example to stop the job

The example catches SIGTERM so it's also possible to run it in the background

`./example bash -c "while true; do echo hello; sleep 2; done" &`

Then use something like `pkill -f example` in order to send the SIGTERM to example and gracefully stop the Job and delete the cgroup.

## Job Logs

Before starting a job it's STDOUT and STDERR are mapped to a file, this ensure the exec.Cmd concurrently writes both outputs to the file. When a client wants to read the logs, it calls `Output(mode)`, where `mode` is either `FollowLogs` or `DontFollowLogs`. If mode is `FollowLogs` then a reader is returned that upon receiving io.EOF, polls the file for changes, waiting for the `pollInterval`. If `DontFollowLogs` is used, then a normal reader is returned. Once a job completes, all of the readers are closed causing any blocking calls to Read to return an error and complete.

Note in production I would use a library to handle the CLI parsing, because of this the `follow` flag for the logs command has to be used like `worker -f logs` instead of `worker logs`.

## Testing

Please see `docs/TESTING.md`