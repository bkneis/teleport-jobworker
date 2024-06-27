.PHONY: lint
lint:
	$(VERBOSE) cd pkg/jobworker && go fmt && go vet

# Build the go application natively
.PHONY: example
example:
	$(VERBOSE) go build -v -o example ./cmd/example

.PHONY: example_debug
example_debug:
	$(VERBOSE) go build -race -tags profiler -v -o example_debug ./cmd/example

.PHONY: concurrent
concurrent:
	$(VERBOSE) go build -race -tags profiler -v -o example_concurrent ./cmd/concurrent

# Build the go application natively
.PHONY: server
server:
	$(VERBOSE) go build -v -o server ./cmd/server

# Build the go application natively
.PHONY: server_debug
server_debug:
	$(VERBOSE) go build -race -tags profiler -v -o server_debug ./cmd/server

# Build the go application natively
.PHONY: client
client:
	$(VERBOSE) go build -v -o client ./cmd/client

.PHONY: proto
proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pkg/proto/worker.proto

# Run the library's unit tests
.PHONY: test
test:
	$(VERBOSE) go test -v -coverprofile coverage.out ./pkg/...

# Run the library's unit tests
.PHONY: integration_test
integration_test:
	$(VERBOSE) go test -tags integration_tests -v -coverprofile coverage.out ./pkg/...

# Run the library's unit tests with race detector
.PHONY: race_test
race_test:
	$(VERBOSE) go test -race -v -coverprofile coverage.out ./pkg/...

# View the code coverage in a web browser
.PHONY: coverage
coverage:
	$(VERBOSE) go tool cover -html=coverage.out

# View the code coverage in a web browser
.PHONY: all
all: server client example