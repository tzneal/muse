presubmit:
	go mod tidy
	go fmt ./...
	go vet ./...
	go test ./...

build:
	go build -o bin/shade ./cmd/shade

install:
	go install ./cmd/shade

clean:
	rm -rf bin/
