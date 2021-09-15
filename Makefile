all: test

test:
	go test -covermode=atomic -coverprofile=coverage.out

.PHONY: all test
