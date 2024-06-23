.PHONY: lint
lint:
	$(VERBOSE) cd pkg/jobworker && go fmt && go vet

# Build the go application natively
.PHONY: example
example:
	$(VERBOSE) go build -v -o example ./cmd/example

# Build the go application natively
.PHONY: example_race
example_race:
	$(VERBOSE) go build -race -v -o example ./cmd/example

.PHONY: proto
proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pkg/proto/worker.proto

# Run the library's unit tests
.PHONY: test
test:
	$(VERBOSE) go test -v -coverprofile coverage.out ./pkg/jobworker/...

# Run the library's unit tests with race detector
.PHONY: race_test
race_test:
	$(VERBOSE) go test -race -v -coverprofile coverage.out ./pkg/jobworker/...

# View the code coverage in a web browser
.PHONY: coverage
coverage:
	$(VERBOSE) go tool cover -html=coverage.out