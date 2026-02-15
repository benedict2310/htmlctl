VERSION ?= dev

.PHONY: build test lint

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/htmlctl ./cmd/htmlctl

test:
	go test ./...

lint:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')
