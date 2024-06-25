# JobWorker
jobworker is a simple golang library that executes arbitrary linux processes with options for resource control using cgroups v2.

The golang library uses the host file system to manage cgroups, where directory and files are created / delete in the cgroup root dir `/sys/fs/cgroup`.

Executing the linux command is executed using `exec.Cmd` where the Cmd is wrapped in a Job struct that provides an API for managing the process.

This repo also contains a simple example of how to use the library. This is not the gRPC client and CLI, just an example usage of the golang library.

The library assumes a 64 bit linux system with cgroups v2, no assurances are provided that the cgroups has correctly performed a request. For instance when creating a group, a directory is created in the cgroup root directory to trigger a group creation, but the library does not perform some sanity check to ensure the cgroup was actually created. Such as stat'ing a file like cgroup.controllers or ensuring it has the supported controllers.

## How to

Build the example

`make example`

or with the race detector and profiler enabled

`make example_debug`

Run concurrent example and view profile information such as number of go routines, heap allocations etc. It simulates 100 clients tailing the logs of one job.

```
make concurrent
./concurrent bash -c "while true; do echo hello; sleep 0.2; done"
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

## Testing cgroups v2

### Using the host
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
todo use io.latency

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

### Using the golang library / example

You should then be able to do the same using the golang library

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

### Testing results on dev machine

Below are some example outputs running on my machine

```
➜  teleport-jobworker git:(feature/v1) ✗ uname -a
Linux marvin 5.15.0-107-generic #117~20.04.1-Ubuntu SMP Tue Apr 30 10:35:57 UTC 2024 x86_64 x86_64 x86_64 GNU/Linux
➜  teleport-jobworker git:(feature/v1) ✗ lscpu
CPU(s):                             8
Thread(s) per core:                 2
Core(s) per socket:                 4
Socket(s):                          1
Model name:                         AMD Ryzen 7 PRO 3700U w/ Radeon Vega Mobile Gfx
...
➜  teleport-jobworker git:(feature/v1) ✗ lsmem 
RANGE                                 SIZE  STATE REMOVABLE  BLOCK
...
Total online memory:      14G
```

Example output for CPU

```
➜  teleport-jobworker git:(feature/v1) ✗ ./example bash -c "stress -q --cpu 8" &
[1] 1128960
Job Status                                                                                                                                                                                     
	ID	     09934f7f-112e-46c0-9b2f-39c32ac015d8
	PID	     1128975
	Running	 true
	ExitCode 0
Job's logs

➜  teleport-jobworker git:(feature/v1) ✗ cat /sys/fs/cgroup/09934f7f-112e-46c0-9b2f-39c32ac015d8/cpu.pressure 
some avg10=0.73 avg60=0.25 avg300=0.05 total=302563
full avg10=0.73 avg60=0.25 avg300=0.05 total=283476

➜  teleport-jobworker git:(feature/v1) ✗ pkill -f example
```

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