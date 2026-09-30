# Wails needs these tags to produce a working desktop window.
DESKTOP_TAGS := desktop,production

# The wails CLI normally adds this; plain `go build` on macOS needs it too.
ifeq ($(shell uname -s),Darwin)
export CGO_LDFLAGS := -framework UniformTypeIdentifiers
endif

build:
	go build -tags $(DESKTOP_TAGS) -o bin/cents ./cmd

run: build
	./bin/cents

run-tui:
	go run ./cmd --mode=t

test:
	go run gotest.tools/gotestsum@latest --format-icons hivis --format-hide-empty-pkg

lint:
	docker run --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v2.13-alpine golangci-lint run

check:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...