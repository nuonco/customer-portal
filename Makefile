# Makefile for installer app
# Uses standalone Tailwind CSS CLI and Air for live reloading

TAILWIND_VERSION := v4.1.18
AIR_VERSION := v1.61.7
RAW_OS := $(shell uname -s)
RAW_ARCH := $(shell uname -m)

# Map OS names to Tailwind's naming convention
ifeq ($(RAW_OS),Darwin)
  OS := macos
else ifeq ($(RAW_OS),Linux)
  OS := linux
else
  OS := windows
endif

# Map architecture names to Tailwind's naming convention
ifeq ($(RAW_ARCH),x86_64)
  ARCH := x64
else ifeq ($(RAW_ARCH),arm64)
  ARCH := arm64
else ifeq ($(RAW_ARCH),aarch64)
  ARCH := arm64
else
  ARCH := x64
endif

TAILWIND_BINARY := tailwindcss-$(OS)-$(ARCH)
TAILWIND_URL := https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/$(TAILWIND_BINARY)

.PHONY: setup setup-tailwind setup-air setup-templ css css-vendor css-customer css-watch dev run clean help generate _tailwind-watch _air-dev _templ-watch

help:
	@echo "Available targets:"
	@echo "  setup        - Install Tailwind CSS CLI + Air + Templ"
	@echo "  generate     - Generate Templ files"
	@echo "  css          - Build all production CSS (minified)"
	@echo "  css-vendor   - Build vendor CSS only"
	@echo "  css-customer - Build customer CSS only"
	@echo "  css-watch    - Watch and rebuild CSS on changes"
	@echo "  dev          - Run with live reload (Air + Tailwind + Templ watch)"
	@echo "  run          - Run Go server only (no live reload)"
	@echo "  clean        - Remove generated files"

setup: setup-tailwind setup-air setup-templ
	@echo ""
	@echo "Setup complete! Run 'make dev' to start development."

setup-tailwind:
	@mkdir -p bin
	@echo "Downloading Tailwind CSS $(TAILWIND_VERSION) standalone CLI..."
	@echo "URL: $(TAILWIND_URL)"
	@curl -sLo bin/tailwindcss $(TAILWIND_URL)
	@chmod +x bin/tailwindcss
	@echo "Done! Tailwind CLI installed at bin/tailwindcss"

setup-air:
	@echo "Installing Air $(AIR_VERSION)..."
	@go install github.com/air-verse/air@$(AIR_VERSION)
	@echo "Done! Air installed."

setup-templ:
	@echo "Installing Templ CLI..."
	@go install github.com/a-h/templ/cmd/templ@latest
	@echo "Done! Templ CLI installed."

# Generate Templ files
generate:
	@echo "Generating Templ files..."
	@templ generate ./internal/views/...
	@echo "Done!"

# Build all CSS files
css: css-vendor css-customer

css-vendor: bin/tailwindcss
	./bin/tailwindcss -i ./src/vendor.css -o ./static/css/vendor.css --minify

css-customer: bin/tailwindcss
	./bin/tailwindcss -i ./src/customer.css -o ./static/css/customer.css --minify

run: generate
	go run main.go

# Internal targets for parallel dev
_tailwind-watch-vendor: bin/tailwindcss
	./bin/tailwindcss -i ./src/vendor.css -o ./static/css/vendor.css --watch

_tailwind-watch-customer: bin/tailwindcss
	./bin/tailwindcss -i ./src/customer.css -o ./static/css/customer.css --watch

_air-dev:
	air

_templ-watch:
	templ generate --watch --proxy="http://localhost:8080" --open-browser=false ./internal/views/...

# Development with live reload
# Runs 3 processes in parallel:
# 1. Air (rebuilds Go on changes, runs on port 8080)
# 2. Tailwind vendor CSS watch
# 3. Tailwind customer CSS watch
#
# Access via: http://localhost:3000
dev: bin/tailwindcss
	@echo "Starting development server with live reload..."
	@echo "- Air Go live reload (port 3000 -> 8080)"
	@echo "- Tailwind CSS watch (vendor + customer)"
	@echo ""
	@echo "Access: http://localhost:3000"
	@echo "Press Ctrl+C to stop"
	@$(MAKE) -j3 _tailwind-watch-vendor _tailwind-watch-customer _air-dev

clean:
	rm -f static/css/vendor.css static/css/customer.css
	find ./internal/views -name "*_templ.go" -delete 2>/dev/null || true
	rm -rf tmp

# Ensure binary exists before CSS targets
bin/tailwindcss:
	@echo "Tailwind CLI not found. Run 'make setup' first."
	@exit 1
