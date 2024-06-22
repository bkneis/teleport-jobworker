.PHONY: lint
lint:
	$(VERBOSE) cd backup && go fmt && go vet

# Build the go application natively
.PHONY: example
example:
	$(VERBOSE) go build -v -o example ./cmd/example

.PHONY: proto
proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pkg/proto/worker.proto

# Run the application's unit tests
.PHONY: test
test:
	$(VERBOSE) go test -v -coverprofile coverage.out ./pkg/jobworker/...

# View the code coverage in a web browser
.PHONY: coverage
coverage:
	$(VERBOSE) go tool cover -html=coverage.out