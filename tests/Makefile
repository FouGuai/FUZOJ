.PHONY: start down connect build status test

start:
	@$(MAKE) down
	@$(MAKE) build
	@python3 ../scripts/debug_start.py

down:
	@python3 ../scripts/debug_stop.py

build:
	@../scripts/build_services.sh

status:
	@ROOT_DIR="$(ROOT_DIR)"; \
	LOG_DIR="$$ROOT_DIR/logs"; \
	services="gateway judge-service submit-service problem-service user-service"; \
	for svc in $$services; do \
		pid_file="$$LOG_DIR/$$svc.pid"; \
		if [ ! -f "$$pid_file" ]; then \
			echo "$$svc: not running (pid file missing)"; \
			continue; \
		fi; \
		pid="$$(tr -d '[:space:]' < "$$pid_file")"; \
		if [ -z "$$pid" ]; then \
			echo "$$svc: not running (pid file empty)"; \
			continue; \
		fi; \
		if kill -0 "$$pid" 2>/dev/null; then \
			echo "$$svc: running (pid $$pid)"; \
		else \
			echo "$$svc: not running (stale pid $$pid)"; \
		fi; \
	done


ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
CONFIG ?= $(ROOT_DIR)/configs/cli.yaml
BASE ?=

connect:
	@if [ -n "$(BASE)" ]; then \
		cd "$(ROOT_DIR)" && go run ./cmd/cli -config "$(CONFIG)" -base "$(BASE)"; \
	else \
		cd "$(ROOT_DIR)" && go run ./cmd/cli -config "$(CONFIG)"; \
	fi

test:
	@$(MAKE) start
	@$(MAKE) test-without-init

test-without-init:
	go test ./... -v -count=1
