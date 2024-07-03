# JobWorker
jobworker is a simple golang library that executes arbitrary linux processes with options for resource control using cgroups v2.

The golang library uses the host file system to manage cgroups, where directory and files are created / delete in the cgroup root dir `/sys/fs/cgroup`.

Executing the linux command is done with `exec.Cmd` where the Cmd is wrapped in a Job struct that provides an API for managing the process.

This repo also contains a simple example of how to use the library. This is not the gRPC client and CLI, just an example usage of the golang library, as well as a example showing support for concurrent readers.

The library assumes a 64 bit linux system with cgroups v2, no assurances are provided that the cgroups are correctly working. For instance when creating a group, a directory is created in the cgroup root directory to trigger a group creation, but the library does not perform some sanity check to ensure the cgroup was actually created.

## How to

Build the example

`make example`

or with the race detector and profiler enabled

`make example_debug`

Run concurrent example and view profile information such as number of go routines, heap allocations etc. It simulates N clients tailing the logs of one job, where N is the first argument of the binary. There should be 1 go routine for each reader and 1 for the job.

```
make concurrent
./concurrent 10 bash -c "while true; do echo hello; sleep 1; done"
google-chrome http://localhost:6060/debug/pprof/
```

Run the libraries unit tests

`make test`

or with the race detector

`make race_test`

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

Before starting a job it's STDOUT and STDERR are mapped to a file, this ensure the exec.Cmd concurrently writes both outputs to the file. When a client wants to read the logs, it calls `Output(mode)`, where `mode` is either `FollowLogs` or `DontFollowLogs`. If mode is `FollowLogs` then a reader is returned that upon recieving io.EOF, polls the file for changes, waiting for the `pollInterval`. If `DontFollowLogs` is used, then a normal reader is returned. Once a job completes, all of the readers are closed causing any blocking calls to Read to return an error and complete.

## Testing cgroups v2

TODO In production and with more time automating some of these tests as a set of integration test would be ideal. Running in a sandbox server with known amounts of compute resources, a series of automated integration tests could run something similar to the example, where stress is executed then the CPU, memory and IO pressure interface file values are validated.

For now I have manually tested the library with cgroups using stress and inspected the pressure files to ensure the `some` and `full` metric show appropriate amounts of wall time.

### How to test cgroups using golang library / example

```
./example bash -c "stress --cpu 2" &

./example bash -c "stress --io 2 --vm 2" &

./example bash -c "dd if=/dev/urandom of=/dev/shm/sample.txt bs=1G count=2 iflag=fullblock" &
```

Check the cgroup PSI files using the Job UUIDs and stop the tests

```
cat /sys/fs/cgroup/{job_uuid}/cpu.pressure
cat /sys/fs/cgroup/{job_uuid}/io.pressure
cat /sys/fs/cgroup/{job_uuid}/memory.pressure

pkill -f example
```

### Test results on dev machine

Below are some example outputs running on my machine (AMD 8 core, 14GB RAM, Ubuntu 20, 5.15.0-107-generic, x86_64 GNU/Linux)

Example output for CPU

```
➜  teleport-jobworker git:(feature/v1) ✗ ./example bash -c "stress -q --cpu 8" &
➜  teleport-jobworker git:(feature/v1) ✗ ./example bash -c "stress -q --cpu 8" &

➜  teleport-jobworker git:(feature/v1) ✗ cat /sys/fs/cgroup/d63e10b8-2059-48fb-9ae2-6c9bd9e521f8/cpu.pressure
some avg10=50.08 avg60=17.03 avg300=4.03 total=12994035
full avg10=32.21 avg60=10.96 avg300=2.59 total=8284627
➜  teleport-jobworker git:(feature/v1) ✗ cat /sys/fs/cgroup/ff30ad27-4dd9-4187-85ef-a8221ecb536d/cpu.pressure
some avg10=50.63 avg60=17.95 avg300=4.30 total=14289995
full avg10=31.15 avg60=11.40 avg300=2.74 total=9041617

➜  teleport-jobworker git:(feature/v1) ✗ pkill -f example
```

Note that the wall time is around 50%, since both jobs try to use 100% of the CPU, they block each other.

Example output for IO

```
➜  teleport-jobworker git:(feature/v1) ✗ ./example bash -c "stress -q --io 2 --vm 2"
Job Status
	ID	     ca7181f4-7c76-4850-b48c-f66b4e35f9e2
	PID	     1138790
	Running	 true
	ExitCode 0
Job's logs

➜  teleport-jobworker git:(feature/v1) ✗ cat /sys/fs/cgroup/ca7181f4-7c76-4850-b48c-f66b4e35f9e2/io.pressure 
some avg10=19.65 avg60=7.25 avg300=1.75 total=5697270
full avg10=17.97 avg60=6.70 avg300=1.62 total=5252884
```

Example output for memory

```
➜  teleport-jobworker git:(feature/v1) ✗ ./example bash -c "dd if=/dev/urandom of=/dev/shm/sample9.txt bs=1G count=2 iflag=fullblock"
Job Status
	ID	     0564cfd1-3224-43f4-ac38-efbe755f0c91
	PID	     1149656
	Running	 true
	ExitCode 0
Job's logs
2+0 records in
2+0 records out
2147483648 bytes (2.1 GB, 2.0 GiB) copied, 23.8283 s, 90.1 MB/s

➜  teleport-jobworker git:(feature/v1) ✗ cat /sys/fs/cgroup/0564cfd1-3224-43f4-ac38-efbe755f0c91/memory.pressure 
some avg10=55.73 avg60=15.45 avg300=3.50 total=11451326
full avg10=55.73 avg60=15.45 avg300=3.50 total=11451326
```

### Using the host

To validate the host's cgroup file and ensure the golang library doesn't act completely different, here are some instructions how to validate cgroups v2 and resource control using just the host. This can be used to verify the golang client or ensure the host is working.

Install stress tool

`apt install stress`

Create test cgroup

`mkdir /sys/fs/cgroup/test && cd /sys/fs/cgroup/test`

Update resource controllers

```bash
echo "100M" > memory.high
echo "100" > cpu.weight
echo "default 100" > io.weight
```

Add process ID

`echo $$ >> cgroup.procs`

Test CPU controller

`stress --cpu 8 &`

then start another terminal session and watch for updates to

`cat cpu.pressure`

Test IO controller

`pkill -f stress && stress --io 2 --vm 2 &`

And watch

`cat io.pressure`

Test memory controller

`pkill -f stress && dd if=/dev/urandom of=/dev/shm/sample.txt bs=1G count=2 iflag=fullblock`

And watch

`cat memory.pressure`

Kill memory test

`pkill -f "dd if=/dev/urandom"`

### Testing Job's linux process model

Below is an example output highlighting the process hierarchy of a Job. Starting a job that spawns child processes, we use `ps` to ensure the correct PPID / PGID is set so that when we stop the Job all child processes are also terminated.

```
➜  teleport-jobworker git:(feature/v1) ✗ ./example_debug bash -c stress --cpu 4 &
➜  teleport-jobworker git:(feature/v1) ✗ ps aux | grep stress
root     1724230  0.1  0.1 2138436 23248 pts/1   Sl+  08:50   0:00 ./example_debug bash -c stress --cpu 4
root     1724256  0.0  0.0   3864   956 pts/1    S+   08:50   0:00 stress --cpu 4
root     1724264  101  0.0   3864   100 pts/1    R+   08:50   0:18 stress --cpu 4
root     1724265  101  0.0   3864   100 pts/1    R+   08:50   0:18 stress --cpu 4
root     1724266  101  0.0   3864   100 pts/1    R+   08:50   0:18 stress --cpu 4
root     1724267  101  0.0   3864   100 pts/1    R+   08:50   0:18 stress --cpu 4
arthur   1724614  0.0  0.0   9044  2564 pts/0    S+   08:51   0:00 grep --color=auto --exclude-dir=.bzr --exclude-dir=CVS --exclude-dir=.git --exclude-dir=.hg --exclude-dir=.svn --exclude-dir=.idea --exclude-dir=.tox stress
➜  teleport-jobworker git:(feature/v1) ✗ ps -f 1724256
UID          PID    PPID  C STIME TTY      STAT   TIME CMD
root     1724256 1724230  0 08:50 pts/1    S+     0:00 stress --cpu 4
➜  teleport-jobworker git:(feature/v1) ✗ ps -o pgid= 1724256     
1723700
```