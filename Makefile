all: build test

build:
	go build $(opts)

test:
	go test $(opts) -covermode=atomic -coverprofile=coverage.out

clean:
	rm coverage.out

.PHONY: all build test clean
