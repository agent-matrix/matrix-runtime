# Matrix Runtime — Makefile
#
# Common targets:
#   make help       list every target
#   make build      build a local binary into ./bin
#   make test       full verification: fmt-check, vet, race tests + coverage
#   make install    build + install the binary (auto-uses sudo when needed)
#   make uninstall  remove an installed binary (and systemd unit if present)
#
# Install without root (user-local, no sudo):
#   make install PREFIX=$HOME/.local
# Production install with systemd + data dir + service user (needs root):
#   sudo make install INSTALL_SYSTEMD=1

SHELL := /usr/bin/env bash

# ---- Module / binary -------------------------------------------------------
MODULE  := github.com/agent-matrix/matrix-runtime
BINARY  := matrix-runtime
PKG     := ./cmd/matrix-runtime
BIN_DIR := bin

# ---- Install locations (GNU-style; honour DESTDIR/PREFIX) ------------------
DESTDIR ?=
PREFIX  ?= /usr/local
BINDIR  ?= $(PREFIX)/bin

# Production install knobs
INSTALL_SYSTEMD ?= 0
SERVICE_USER    ?= matrix
DATA_DIR        ?= /var/lib/matrix-runtime
SYSTEMD_DIR     ?= /etc/systemd/system
ENV_FILE        ?= /etc/matrix-runtime/matrix-runtime.env

# ---- Version stamping ------------------------------------------------------
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(MODULE)/internal/config.Version=$(VERSION) \
	-X $(MODULE)/internal/config.Commit=$(COMMIT) \
	-X $(MODULE)/internal/config.Date=$(DATE)

# Static, reproducible production build.
GOFLAGS_PROD := -trimpath
CGO_ENABLED  ?= 0

GO_FILES := $(shell find . -name '*.go' -not -path './legacy/*')

.DEFAULT_GOAL := build
# Run a shell script robustly even if a Windows/WSL checkout gave it CRLF line
# endings: strip CR first, then run with bash (no exec-bit dependency).
RUNSH = sh_strip_run() { sed -i 's/\r$$//' "$$1" 2>/dev/null || true; bash "$$1" "$${@:2}"; }; sh_strip_run

.PHONY: all build prod-build run test fmt fmt-check vet lint tidy coverage \
        docker compose-up compose-down smoke install uninstall \
        install-systemd release clean help web web-auto normalize \
        venv py-install py-test py-lint setup

## all: build the frontend bundle and the backend binary
all: web prod-build

## normalize: strip CRLF -> LF from scripts, Makefile and Go (Windows/WSL fix)
normalize:
	@for f in $$(find . -path ./legacy -prune -o \( -name '*.sh' -o -name '*.go' \) -print) Makefile; do \
		sed -i 's/\r$$//' "$$f" 2>/dev/null || true; \
	done
	@echo "normalize: line endings set to LF"

## web: build the enterprise console bundle (web/src -> web/static/app.js)
web:
	@$(RUNSH) scripts/build-web.sh

# web-auto: rebuild the console bundle when a JS toolchain is available, else
# fall back to the committed web/static/app.js (already embedded in the binary).
# Best-effort: never fails the build (works offline / air-gapped / CRLF checkout).
web-auto:
	@if command -v npx >/dev/null 2>&1; then \
		$(RUNSH) scripts/build-web.sh || echo "web build failed — using committed web/static/app.js"; \
	else \
		echo "==> npx not found — using committed frontend (web/static/app.js, embedded)"; \
	fi

## build: compile a local binary into ./bin (backend API + embedded console)
build:
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(PKG)
	@echo "built $(BIN_DIR)/$(BINARY) ($(VERSION))"

## prod-build: optimized, static, version-stamped binary
prod-build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS_PROD) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(PKG)
	@echo "built $(BIN_DIR)/$(BINARY) ($(VERSION), static)"

## run: start the whole MatrixCloud (API + console + SQLite) in local-dev mode
run: web-auto
	@echo "==> MatrixCloud starting — the console URL is printed below"
	@echo "    (if port 8080 is busy it falls back to the next free port; override with MATRIX_RUNTIME_PORT)"
	go run -ldflags "$(LDFLAGS)" $(PKG) --mode local-dev

## test: full verification gate (fmt, vet, race tests + coverage)
test: fmt-check vet
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

## fmt: normalise line endings (CRLF->LF) and gofmt all non-legacy Go files
fmt:
	@for f in $(GO_FILES); do sed -i 's/\r$$//' "$$f" 2>/dev/null || true; done
	@gofmt -w $(GO_FILES)
	@echo "gofmt: formatted $(words $(GO_FILES)) files (LF normalised)"

## fmt-check: fail if any Go file has CRLF endings or is not gofmt-clean
fmt-check:
	@crlf="$$(grep -lUP '\r$$' $(GO_FILES) 2>/dev/null || true)"; \
	if [ -n "$$crlf" ]; then \
		echo "These files have CRLF (Windows) line endings — gofmt requires LF:"; \
		echo "$$crlf" | sed 's/^/  /'; \
		echo ""; \
		echo "Fix:  make fmt"; \
		echo "This repo ships .gitattributes to keep LF; if git re-introduces CRLF run:"; \
		echo "      git add --renormalize . && git checkout ."; \
		exit 1; \
	fi; \
	unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-clean:"; echo "$$unformatted" | sed 's/^/  /'; \
		echo "Run 'make fmt'."; exit 1; \
	fi; \
	echo "gofmt: clean"

## vet: run go vet
vet:
	go vet ./...

## lint: run golangci-lint if installed (optional)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; skipping (using 'go vet' via 'make vet')"; \
	fi

## tidy: ensure go.mod is tidy
tidy:
	go mod tidy

## coverage: open a coverage report
coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "wrote coverage.html"

# ---- Python client (clients/python) — uv-managed .venv ---------------------
PY_DIR := clients/python

## venv: create clients/python/.venv with uv (fast) and install the client
venv:
	@if command -v uv >/dev/null 2>&1; then \
		cd $(PY_DIR) && uv sync --extra dev && \
		echo "venv ready: $(PY_DIR)/.venv  (run: source $(PY_DIR)/.venv/bin/activate)"; \
	else \
		echo "uv not found — install it for fast setup: https://docs.astral.sh/uv/"; \
		echo "falling back to python venv + pip…"; \
		cd $(PY_DIR) && python3 -m venv .venv && ./.venv/bin/pip install -e '.[dev]'; \
	fi

## py-install: install the matrixcloud client + CLI into the .venv
py-install: venv

## py-test: run the Python client test suite
py-test:
	@cd $(PY_DIR) && if command -v uv >/dev/null 2>&1; then uv run pytest; else ./.venv/bin/pytest; fi

## py-lint: lint the Python client (ruff)
py-lint:
	@cd $(PY_DIR) && if command -v uv >/dev/null 2>&1; then uv run ruff check .; else ./.venv/bin/ruff check .; fi

## setup: build the runtime AND set up the Python client venv (full dev setup)
setup: build venv
	@echo "setup complete — run 'make run' to start MatrixCloud, or 'cd $(PY_DIR) && uv run mxc status'"

## docker: build the container image
docker:
	docker build -t $(BINARY):$(VERSION) -t $(BINARY):local .

## compose-up / compose-down: docker compose helpers
compose-up:
	docker compose -f deploy/docker-compose/docker-compose.yml up --build

compose-down:
	docker compose -f deploy/docker-compose/docker-compose.yml down

## smoke: run the smoke test against a running instance
smoke:
	@$(RUNSH) scripts/smoke-test.sh

## install: install the runtime (backend API + embedded MatrixCloud console).
##          Auto-elevates with sudo when the target dir needs root. Use
##          PREFIX=$HOME/.local for a user install with no root. Set
##          INSTALL_SYSTEMD=1 (as root) to also install a systemd service.
install: web-auto prod-build
	@bindir="$(DESTDIR)$(BINDIR)"; sudo=""; \
	if [ "$$(id -u)" = "0" ] || [ -w "$$(dirname "$$bindir")" ] || [ -w "$$bindir" ]; then \
		: ; \
	elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then \
		sudo="sudo"; echo "==> $$bindir needs root — using passwordless sudo"; \
	else \
		bindir="$$HOME/.local/bin"; \
		echo "==> $(BINDIR) needs root and passwordless sudo is unavailable —"; \
		echo "    installing to $$bindir instead (override with PREFIX=... or run: sudo make install)"; \
	fi; \
	echo "==> installing $(BINARY) (backend + embedded console) to $$bindir"; \
	$$sudo install -d "$$bindir" && \
	$$sudo install -m 0755 "$(BIN_DIR)/$(BINARY)" "$$bindir/$(BINARY)" && \
	echo "installed: $$bindir/$(BINARY)"; \
	case ":$$PATH:" in *":$$bindir:"*) ;; *) echo "note: add $$bindir to your PATH — e.g.  echo 'export PATH=\"$$bindir:\$$PATH\"' >> ~/.bashrc";; esac; \
	echo "run it:  $(BINARY) --mode local-dev   # then open http://localhost:8080"
	@if [ "$(INSTALL_SYSTEMD)" = "1" ]; then $(MAKE) install-systemd; else \
		echo "(systemd service available via: sudo make install INSTALL_SYSTEMD=1)"; fi

## install-systemd: provision service user, data dir, env file and unit
install-systemd:
	@echo "==> provisioning systemd service"
	@id -u "$(SERVICE_USER)" >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin "$(SERVICE_USER)"
	install -d -o "$(SERVICE_USER)" -g "$(SERVICE_USER)" "$(DESTDIR)$(DATA_DIR)"
	install -d "$(DESTDIR)$(dir $(ENV_FILE))"
	@if [ ! -f "$(DESTDIR)$(ENV_FILE)" ]; then \
		install -m 0640 deploy/systemd/matrix-runtime.env.example "$(DESTDIR)$(ENV_FILE)"; \
		echo "installed env file: $(DESTDIR)$(ENV_FILE) (edit before starting)"; \
	else echo "env file exists, leaving as-is: $(DESTDIR)$(ENV_FILE)"; fi
	install -d "$(DESTDIR)$(SYSTEMD_DIR)"
	sed -e "s|@BINDIR@|$(BINDIR)|g" \
	    -e "s|@ENV_FILE@|$(ENV_FILE)|g" \
	    -e "s|@DATA_DIR@|$(DATA_DIR)|g" \
	    -e "s|@SERVICE_USER@|$(SERVICE_USER)|g" \
	    deploy/systemd/matrix-runtime.service.in > "$(DESTDIR)$(SYSTEMD_DIR)/matrix-runtime.service"
	@echo "installed unit: $(DESTDIR)$(SYSTEMD_DIR)/matrix-runtime.service"
	@echo "next: sudo systemctl daemon-reload && sudo systemctl enable --now matrix-runtime"

## uninstall: remove the installed binary (and systemd unit if present)
uninstall:
	@bindir="$(DESTDIR)$(BINDIR)"; sudo=""; \
	if [ "$$(id -u)" != "0" ] && [ -e "$$bindir/$(BINARY)" ] && [ ! -w "$$bindir" ]; then \
		command -v sudo >/dev/null 2>&1 && sudo="sudo"; fi; \
	$$sudo rm -f "$$bindir/$(BINARY)"; \
	if [ -f "$(DESTDIR)$(SYSTEMD_DIR)/matrix-runtime.service" ]; then \
		$$sudo rm -f "$(DESTDIR)$(SYSTEMD_DIR)/matrix-runtime.service"; \
		echo "removed systemd unit (run: sudo systemctl daemon-reload)"; \
	fi; \
	echo "uninstalled $(BINARY)"

## release: cross-compile static binaries into ./dist
release:
	@mkdir -p dist
	@for osarch in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		os=$${osarch%/*}; arch=$${osarch#*/}; \
		out="dist/$(BINARY)-$$os-$$arch"; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build $(GOFLAGS_PROD) -ldflags "$(LDFLAGS)" -o "$$out" $(PKG) || exit 1; \
	done
	@echo "release artifacts in ./dist"

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) dist coverage.out coverage.html

## help: list targets
help:
	@printf "\n  \033[1;32mMatrix Runtime\033[0m — the execution plane for Matrix Cloud\n"
	@printf "  backend API + embedded MatrixCloud console + SQLite, in one binary\n\n"
	@printf "  \033[1mTargets\033[0m\n"
	@grep -E '^## [a-z][a-z-]*:' $(MAKEFILE_LIST) \
		| sed -E 's/^## ([a-z-]+): /\1|/' \
		| awk -F'|' '{printf "    \033[36m%-14s\033[0m %s\n", $$1, $$2}'
	@printf "\n  \033[1mQuick start\033[0m\n"
	@printf "    make run                     start everything → http://localhost:8080\n"
	@printf "    make build                   build bin/matrix-runtime\n"
	@printf "    make test                    fmt-check + vet + race tests + coverage\n"
	@printf "    sudo make install            install to /usr/local/bin\n"
	@printf "    make install PREFIX=\$$HOME/.local   user install, no root\n\n"
