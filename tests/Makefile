.PHONY: start down connect build status test killall

start:
	@$(MAKE) down
	@$(MAKE) build
	@python3 ../scripts/debug_start.py --deps-only
	@go run ../scripts/devtools/etcdinit
	@python3 ../scripts/debug_start.py --services-only

down:
	@python3 ../scripts/debug_stop.py

killall:
	@bash ./killall.sh

build:
	@../scripts/build_services.sh

status:
	@ROOT_DIR="$(ROOT_DIR)" python3 ../scripts/status_services.py


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
