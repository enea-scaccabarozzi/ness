.PHONY: validate build lint test audit vuln selfcheck

# The gate (go-strict): every step must pass. Run it before every push.
validate: build lint test audit vuln selfcheck

build:
	CGO_ENABLED=0 go build ./...

lint:
	golangci-lint run --allow-serial-runners ./... || { echo 'lint failed (skill ness-test-discipline, section Rules)'; exit 1; }
	golangci-lint run --allow-serial-runners -c .golangci.tests.yml ./... || { echo 'test lint failed (skill ness-test-discipline, section Rules)'; exit 1; }

test:
	go test -race -count=1 ./...
	go test -count=4 ./...

audit:
	python3 .ness/audit.py

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

selfcheck:
	ness sync --check
