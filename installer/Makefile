GO ?= go
VERSION ?= 1.2.0
COMMIT ?= unknown
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BINARY ?= otter-install
LDFLAGS ?= -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)

.PHONY: build sync-assets test vet check clean

sync-assets:
	mkdir -p internal/assets/workflows internal/assets/configs
	cp -R ../craftmake/workflows/. internal/assets/workflows/
	cp -R ../craftmake/configs/. internal/assets/configs/

build: sync-assets
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test: sync-assets
	$(GO) test ./...

vet:
	$(GO) vet ./...

check: test vet

clean:
	rm -f $(BINARY)
