BINARY := helm-values-checker
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/chrishham/helm-values-checker/cmd.version=$(VERSION) \
	-X github.com/chrishham/helm-values-checker/cmd.commit=$(COMMIT) \
	-X github.com/chrishham/helm-values-checker/cmd.date=$(DATE)

.PHONY: build test lint clean install snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) .

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ dist/

install: build
	mkdir -p $(HELM_PLUGINS)/values-checker/bin
	cp bin/$(BINARY) $(HELM_PLUGINS)/values-checker/bin/
	cp plugin.yaml $(HELM_PLUGINS)/values-checker/

snapshot:
	goreleaser release --snapshot --clean
