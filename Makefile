VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: web build run test lint vet clean docker release

web:
	cd web && npm ci && npm run build

build: web
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/shortr ./cmd/shortr

build-go-only:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/shortr ./cmd/shortr

test:
	go test ./... -race -count=1

vet:
	go vet ./...

lint: vet
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

clean:
	rm -rf bin web/dist

docker:
	docker build -t shortr:$(VERSION) --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) .

run: build
	SHORTR_DATA_DIR=./data ./bin/shortr

# cross-compile matrix for release artifacts
release: web
	mkdir -p dist
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/shortr-linux-amd64   ./cmd/shortr
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/shortr-linux-arm64   ./cmd/shortr
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/shortr-darwin-amd64  ./cmd/shortr
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/shortr-darwin-arm64  ./cmd/shortr
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o dist/shortr-windows-amd64.exe ./cmd/shortr
	cd dist && sha256sum * > SHA256SUMS
