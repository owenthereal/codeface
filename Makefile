
.PHONY: build
build:
	go build -o bin/cf-client ./cmd/cf-client
	go build -o bin/cf-server ./cmd/cf-server
