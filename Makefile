GO_DAEMON_DIR := ./daemon

.PHONY: daemon-run daemon-build daemon-test

daemon-run:
	cd $(GO_DAEMON_DIR) && go run ./cmd/dope

daemon-build:
	cd $(GO_DAEMON_DIR) && go build ./cmd/dope

daemon-test:
	cd $(GO_DAEMON_DIR) && go test ./...
