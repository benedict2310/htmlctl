VERSION ?= dev

.PHONY: build test lint

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/htmlctl ./cmd/htmlctl
	go build -ldflags "-X main.version=$(VERSION)" -o bin/htmlservd ./cmd/htmlservd

test:
	go test ./...

lint:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')
