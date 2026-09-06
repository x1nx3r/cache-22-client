.PHONY: test vet bindings css build dev

test:
	go test ./...

vet:
	go vet ./...

bindings:
	wails3 generate bindings

build:
	wails3 build

dev:
	CACHE22_PROFILE=1 wails3 dev -config ./build/config.yml
