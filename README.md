# JobWorker
jobworker is a simple golang library &amp; gRPC server / client that executes arbitrary linux processes with options for resource control using cgroups v2.

The golang library uses the host file system to manage cgroups, where directory and files are created / delete in the cgroup root dir `/sys/fs/cgroup`.

Executing the linux command is done using `exec.Cmd` where the command is wrapped in a bash session that adds it's PID to the cgroup before executing the command.

This repo also contains a simple example of how to use the library. This is not the gRPC client and CLI, just an example usage of the golang library.

The library assumes a 64 bit linux system with cgroups v2, no assurances are provided that the cgroups has correctly performed a request. For instance when creating a group, a directory is created in the cgroup root directory to trigger a group creation, but the library does not perform some sanity check to ensure the cgroup was actually created. Such as stat'ing a file like cgroup.controllers.

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

## Testing cgroups v2

Install stress tool

`apt install stress`

Create test cgroup

`mkdir /sys/fs/cgroup/test && cd /sys/fs/cgroup/test`

Update resource controllers

```bash
echo "100M" > memory.high
echo "200M" > memory.max
echo "100" > cpu.weight
echo "100000 1000000" > cpu.max
echo "default 100" > io.weight
```
todo use io.latency

Add process ID

`echo $$ >> /sys/fs/cgroup/test/cgroup.procs`

Start another terminal session and watch for updates to

`cpu.pressure`

Test CPU controller

`stress --cpu 8`

And ensure the stall time for the averages are not 0

Test IO controller

`stress --io 2 --vm 2`

And watch

`io.pressure`

And ensure the stall time for the averages are not 0

Test memory controller

`dd if=/dev/urandom of=/dev/shm/sample.txt bs=1G count=2 iflag=fullblock`

And watch

`memory.pressure`

And ensure the stall time for the averages are not 0

You should then be able to do the same using the golang library

./example bash -c "apt install -y stress && stress --cpu 2"

./example bash -c "apt install -y stress && stress --io 2 --vm 2"

Check the cgroup files using Job UUID

`export job_uuid=`
`cat /sys/fs/cgroup/${job_uuid}/cpu.pressure`
`cat /sys/fs/cgroup/${job_uuid}/io.pressure`
`cat /sys/fs/cgroup/${job_uuid}/memory.pressure`