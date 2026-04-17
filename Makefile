.PHONY: build test clean

build:
	go build -o bin/taskhouse-server ./cmd/server
	go build -o bin/task ./cmd/task

test:
	go test ./...

clean:
	rm -rf bin/
