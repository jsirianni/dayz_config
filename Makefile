.PHONY: build check
build:
	go build -o .tools/bin/ ./cmd/dzdeploy

check:
	go vet ./...
	go test ./...

.PHONY: test
test:
	go test ./...

.PHONY: tidy
tidy:
	go mod tidy
