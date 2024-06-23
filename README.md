# JobWorker
jobworker is a simple golang library &amp; gRPC server / client that executes arbitrary linux processes with options for resource control using cgroups v2.

The golang library uses the host file system to manage cgroups, where directory and files are created / delete in the cgroup root dir `/sys/fs/cgroup`.

Executing the linux command is done using `exec.Cmd` where the command is wrapped in a bash session that adds it's PID to the cgroup before executing the command.

This repo also contains a simple example of how to use the library. This is not the gRPC client and CLI, just an example usage of the golang library.

## How to

Build the example
`make example`

or with the race detector enabled
`make example_race`

Run the libraries unit tests
`make test`

or with the race detector
`make race_test`

Generate the protobuf go & grpc definitions after updating `pkg/proto/worker.proto`

`make proto`

Lint the code for static analysis
`make lint`

Example long lived command with tailing

`./worker "while true; do echo hello; sleep 2; done"`

Ctrl+C to signal example to stop the job

View test coverage
`make coverage`