# Makefile for TS-timeline-system
# Helpful targets for local development and testing


.PHONY: help e2e e2e-full


help:
	@echo "Available targets:"
	@echo "  e2e      Run the comprehensive e2e test with cleanup (scripts/test_all_curl_and_cleanup.sh)"
	@echo "  e2e-full Run the old lightweight e2e_smoke.sh (deprecated)"

# Run the end-to-end smoke test

e2e:
	@command -v jq >/dev/null 2>&1 || { echo >&2 "jq is required but not installed. Install with: brew install jq"; exit 1; }
	@command -v curl >/dev/null 2>&1 || { echo >&2 "curl is required but not installed."; exit 1; }
	@echo "Running comprehensive end-to-end test (with cleanup)..."
	@./scripts/test_all_curl_and_cleanup.sh

e2e-full:
	@echo "Running legacy e2e_smoke.sh (if present)"
	@./scripts/e2e_smoke.sh || echo "legacy script not found"

loadtest:
	@command -v go >/dev/null 2>&1 && (echo "Running Go loadtest..." && go run ./tools/loadtest) || \
	(echo "Go is not installed or not in PATH. To run the loadtest: install Go and run: go run ./tools/loadtest/main.go" )

loadtest-bash:
	@command -v jq >/dev/null 2>&1 || { echo >&2 "jq is required but not installed. Install with: brew install jq"; exit 1; }
	@command -v curl >/dev/null 2>&1 || { echo >&2 "curl is required but not installed."; exit 1; }
	@echo "Running bash loadtest..."
	@./scripts/loadtest_bash.sh
