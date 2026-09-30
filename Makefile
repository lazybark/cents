test:
	go run gotest.tools/gotestsum@latest --format-icons hivis --format-hide-empty-pkg

lint:
	docker run --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v2.12-alpine golangci-lint run

check:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...