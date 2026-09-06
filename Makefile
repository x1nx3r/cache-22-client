.PHONY: tidy build run test vet fmt clean-cache

clean-cache:
	@if [ -z "$(SERIAL)" ]; then echo 'usage: make clean-cache SERIAL="<serial>|all"'; exit 1; fi
	go run ./cmd/cache22 clean "$(SERIAL)"

tidy:
	go mod tidy

build:
	go build -o bin/cache22 ./cmd/cache22

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...
