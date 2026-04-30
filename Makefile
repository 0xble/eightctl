.PHONY: fmt lint test coverage

fmt:
	go tool mvdan.cc/gofumpt -w ./

lint:
	golangci-lint run ./...

test:
	go test ./...

coverage:
	go test -coverpkg=./internal/client,./internal/config,./internal/daemon,./internal/output,./internal/tokencache ./internal/client ./internal/config ./internal/daemon ./internal/output ./internal/tokencache -coverprofile=coverage.out
	go tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $$3); print "core coverage: " $$3 "%"; exit !($$3 >= 85.0)}'
