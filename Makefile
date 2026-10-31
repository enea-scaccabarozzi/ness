FUZZTIME ?= 30s

.PHONY: validate build lint test repeat audit vuln selfcheck fuzz e2e eval-autonomy eval-delegation

validate: build lint test audit vuln selfcheck

build:
	CGO_ENABLED=0 go build ./...
	go vet -tags e2e ./internal/cli/ ./internal/atom/... # the opt-in e2e files still compile

lint:
	golangci-lint run --allow-serial-runners ./...
	golangci-lint run --allow-serial-runners -c .golangci.tests.yml ./...

test:
	go test -race -count=1 ./...

# Flaky-test hunt, CI only (R-QA-009): it doubles the gate's time.
repeat:
	go test -count=4 ./...

audit:
	python3 .ness/audit.py

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

selfcheck:
	go run ./cmd/ness sync --check

fuzz:
	@for pkg in $$(go list ./...); do \
	  for f in $$(go test -list '^Fuzz' $$pkg | grep '^Fuzz'); do \
	    go test -run '^$$' -fuzz "^$$f$$" -fuzztime $(FUZZTIME) $$pkg || exit 1; \
	  done; \
	done

# Real-client runs: inside herdr, opt-in, stopped by the weekly-usage rule (D31, R-QA-027).
# -timeout is finite but above 5 tests x (8m test + 2m cleanup): the binary's panic would skip every cleanup.
e2e:
	NESS_E2E=1 go test -tags e2e -timeout 60m -run 'Real' ./internal/cli/ ./internal/atom/... -v -count=1

# Autonomy text on the real client (claude -p), opt-in and billed: about $0.3 a run, 30 runs (R-PROF-080).
EVAL_OUT ?= $(shell mktemp -d)
eval-autonomy:
	python3 evals/autonomy/eval.py $(EVAL_ARGS) $(EVAL_OUT)

eval-delegation:
	python3 evals/delegation/eval.py $(EVAL_ARGS) $(EVAL_OUT)
