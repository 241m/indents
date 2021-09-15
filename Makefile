all: test

test:
	go test -covermode=atomic -coverprofile=coverage.out

clean:
	rm coverage.out

.PHONY: all test clean
