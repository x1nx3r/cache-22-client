.PHONY: tidy build run test vet fmt clean-cache release-cli

RELEASE_ARCHS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Cross-compiles the CLI for release. No windows targets: the CLI links
# go-fuse, which has no Windows port. The GUI packages come from
# `cd gui && wails3 task package` (needs the webkit dev toolchain + nfpm).
release-cli:
	@mkdir -p dist
	@for a in $(RELEASE_ARCHS); do \
		os=$${a%%/*}; arch=$${a##*/}; \
		out=dist/cache22-$${os}-$${arch}; \
		if [ "$$os" = "windows" ]; then out=$${out}.exe; fi; \
		echo "== $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-w -s" -o $$out ./cmd/cache22 || exit 1; \
	done
	@ls -la dist/

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
