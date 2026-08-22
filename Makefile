SHELL := /bin/bash
.DEFAULT_GOAL := ci

.PHONY: ci fmt lint test release publish deploy

ci: fmt lint test

fmt:
	@test -z "$$(gofmt -l ./cmd ./internal ./models ./pkg)" || { \
		gofmt -l ./cmd ./internal ./models ./pkg >&2; \
		exit 1; \
	}
	git diff --check

lint:
	go vet ./...

test:
	go test ./...

release publish deploy:
	@application_root="$$(git rev-parse --show-toplevel)"; \
		gateway_root="$$(dirname "$${application_root}")/mprlab-gateway"; \
		$(MAKE) --no-print-directory -C "$${gateway_root}" "app-$@" \
			MPRLAB_APP_ROOT="$${application_root}"
