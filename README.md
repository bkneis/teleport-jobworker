# teleport-jobworker
jobworker is a simple golang library &amp; gRPC server / client that executes arbitrary linux processes with options for resource control using cgroups v2

## How to

Build the example
`make build`

Run the libraries unit tests
`make test`

Generate the protobuf go & grpc definitions after updating `pkg/proto/worker.proto`
`make proto`

Lint the code for static analysis
`make lint`
