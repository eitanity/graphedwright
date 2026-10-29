# `make ci` runs the every-push checks. The workflows call these targets.
# Tool versions are pinned here.

GOLANGCI_LINT_VERSION := v2.14.0
GOIMPORTS_VERSION     := v0.52.0
GOVULNCHECK_VERSION   := v1.9.0
TINYGO_VERSION        := 0.42.0
BIN                   := $(abspath bin)
PKGS                  := ./...
HOTPATH               := internal/lint/hotpath.txt
WASM_SIZE_LIMIT_GO    := 4194304
WASM_SIZE_LIMIT_TINY  := 614400

.PHONY: all ci tools verify build tinygo-vet fmt vet lint custom-lint escapes nofma alloc test race fuzz cover vuln wasm wasm-smoke wasm-test qemu-test merilo demo size golden

all: ci

ci: verify build tinygo-vet fmt vet lint custom-lint escapes nofma race alloc fuzz merilo cover vuln wasm-test size

# ---- toolchain ------------------------------------------------------------

tools: $(BIN)/golangci-lint $(BIN)/goimports $(BIN)/govulncheck $(BIN)/graphedvet $(BIN)/norecursion $(BIN)/escapes
	@tinygo version 2>/dev/null | grep -q "tinygo version $(TINYGO_VERSION)" || { echo "tinygo $(TINYGO_VERSION) is required: https://tinygo.org/getting-started/install/"; exit 1; }

$(BIN)/golangci-lint:
	GOBIN=$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(BIN)/goimports:
	GOBIN=$(BIN) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

$(BIN)/govulncheck:
	GOBIN=$(BIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

$(BIN)/graphedvet $(BIN)/norecursion $(BIN)/escapes: $(wildcard internal/lint/*/*.go internal/lint/*/*/*.go)
	cd internal/lint && go build -o $(BIN)/graphedvet ./cmd/graphedvet \
	  && go build -o $(BIN)/norecursion ./norecursion \
	  && go build -o $(BIN)/escapes ./escapes

verify:
	go mod verify
	go mod tidy -diff
	cd internal/lint && go mod verify && go mod tidy -diff
	@bad="$$(git ls-files -z | xargs -0 file --mime-type | grep -E 'application/(x-executable|x-pie-executable|x-sharedlib|x-mach-binary|x-dosexec|wasm|x-object|x-archive)' | cut -d: -f1)"; \
	  if [ -n "$$bad" ]; then echo "tracked binary files:"; echo "$$bad"; exit 1; fi

# Every package builds for the host, for the browser and for WASI with
# the Go toolchain.
build:
	go build -trimpath $(PKGS)
	GOOS=js GOARCH=wasm go build -trimpath $(PKGS)
	GOOS=wasip1 GOARCH=wasm go build -trimpath $(PKGS)

# Every package with tests compiles with TinyGo, tests included, so that
# anything outside the TinyGo subset fails here and not in the browser.
# A package without tests is skipped and named; every package gets tests.
tinygo-vet:
	@tinygo version | grep -q "tinygo version $(TINYGO_VERSION)" || { echo "tinygo $(TINYGO_VERSION) is required"; exit 1; }
	@for p in $$(go list -f '{{.Dir}}' $(PKGS)); do \
	  rel=$$(realpath --relative-to=. $$p); \
	  if [ -z "$$(go list -f '{{.TestGoFiles}}{{.XTestGoFiles}}' ./$$rel | tr -d '[]')" ]; then echo "tinygo: ./$$rel has no tests yet, skipped"; continue; fi; \
	  tinygo test -c -o /dev/null -target wasip1 ./$$rel || exit 1; \
	done

# ---- formatting, vet, lint ------------------------------------------------

fmt: $(BIN)/goimports
	@out="$$(gofmt -l . ; $(BIN)/goimports -l .)"; if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

vet:
	go vet $(PKGS)
	cd internal/lint && go vet ./...
	GOOS=js GOARCH=wasm go vet $(PKGS)
	@! grep -rn --include='*.go' --exclude-dir=bin --exclude-dir=.git '^//go:build' . || { echo "build constraints are not allowed"; exit 1; }
	@bad="$$(find . -path ./bin -prune -o -name '*.go' -print | grep -E '_(js|wasm|wasip1|linux|darwin|windows|amd64|arm64)(_test)?\.go$$' | grep -v -E '^\./(dom|cmd/graphedwright-wasm)/')"; \
	  if [ -n "$$bad" ]; then echo "only dom/ and cmd/graphedwright-wasm/ may hold target-named files:"; echo "$$bad"; exit 1; fi

lint: $(BIN)/golangci-lint
	$(BIN)/golangci-lint run $(PKGS)
	GOOS=js GOARCH=wasm $(BIN)/golangci-lint run $(PKGS)

custom-lint: $(BIN)/graphedvet $(BIN)/norecursion
	go vet -vettool=$(BIN)/graphedvet $(PKGS)
	GOOS=js GOARCH=wasm go vet -vettool=$(BIN)/graphedvet $(PKGS)
	$(BIN)/norecursion -wrappers internal/lint/wrappers.txt $(PKGS)
	cd internal/lint && go test ./...

escapes: $(BIN)/escapes
	go build -gcflags=-m $(PKGS) 2>&1 | $(BIN)/escapes $(HOTPATH)

# No float64 expression is fused into a multiply-add on any target. The
# Go spec lets arm64, riscv64, loong64, ppc64 and s390x fuse x*y + z,
# which rounds once, so a fused line gives a different result there than
# on amd64 and wasm. An explicit float64(x*y) rounds the product and
# forbids the fusion.
FMA_ARCHS := arm64 riscv64 loong64 ppc64le s390x

nofma:
	@out="$$(for a in $(FMA_ARCHS); do \
	  GOARCH=$$a go build -gcflags=-S $(PKGS) 2>&1 | grep -E '\bFN?M(ADD|SUB)[DS]?\b' | grep -oE '[^ (]+\.go:[0-9]+' | sed "s|^$(CURDIR)/||; s|^|$$a |"; \
	done | sort -u)"; \
	if [ -n "$$out" ]; then echo "fused multiply-adds; wrap the product in float64(...):"; echo "$$out"; exit 1; fi

# ---- tests ------------------------------------------------------------------

test:
	go test -count=1 $(PKGS)

race:
	GRAPHEDWRIGHT_RACE=1 go test -race -count=1 -timeout 30m $(PKGS)

alloc:
	go test -count=1 -run 'Alloc|Churn' $(PKGS)

# Every fuzz target natively, then on 386, where an int has 32 bits as
# in TinyGo's wasm. Go guides fuzzing by coverage only on amd64 and
# arm64, so the 386 run mutates without it. Its workers are capped: each
# shares 100 MB with the coordinator, and 32 of them overrun a 32-bit
# address space. internal/verify is its own module; make merilo fuzzes it.
fuzz:
	@for f in $$(grep -rl --include='*_test.go' --exclude-dir=verify '^func Fuzz' . | xargs -r dirname | sort -u); do \
	  for name in $$(grep -ho '^func Fuzz[A-Za-z0-9_]*' $$f/*_test.go | sed 's/^func //'); do \
	    go test -run='^$$' -fuzz="^$$name$$" -fuzztime=60s $$f || exit 1; \
	    GOARCH=386 go test -run='^$$' -fuzz="^$$name$$" -fuzztime=60s -parallel=16 $$f || exit 1; \
	  done; \
	done

# The one coverage gate: the document decoder's every path, error paths
# included, is reached by a test.
cover:
	@go test -count=1 -coverprofile=$(BIN)/doc.cov ./doc >/dev/null && \
	  pct=$$(go tool cover -func=$(BIN)/doc.cov | tail -1 | grep -o '[0-9.]*%' | tr -d %) && \
	  echo "doc coverage $$pct%" && test "$$pct" = "100.0" || { echo "the doc package must be fully covered"; exit 1; }

vuln: $(BIN)/govulncheck
	$(BIN)/govulncheck $(PKGS)

# ---- browser builds -----------------------------------------------------------

# Both toolchains build the browser entry point; size records what shipped.
# Each build is redone only when a file it compiles or embeds, the module
# files or this Makefile has changed since.
WASM_SRCS := $(shell GOOS=js GOARCH=wasm go list -deps -f '{{if not .Standard}}{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .EmbedFiles}}{{$$.Dir}}/{{.}} {{end}}{{end}}' ./cmd/graphedwright-wasm) go.mod $(wildcard go.sum) Makefile

wasm: $(BIN)/graphedwright.go.wasm $(BIN)/graphedwright.tinygo.wasm

$(BIN)/graphedwright.go.wasm: $(WASM_SRCS)
	GOOS=js GOARCH=wasm go build -trimpath -o $@ ./cmd/graphedwright-wasm

$(BIN)/graphedwright.tinygo.wasm: $(WASM_SRCS)
	tinygo build -o $@ -target wasm -opt=z -no-debug ./cmd/graphedwright-wasm

# The browser smoke test under each toolchain, in headless Firefox
# through geckodriver: the page's export, drawing, annotation, DOM and
# refusals against the native command's render of the same document.
wasm-smoke: wasm
	go run ./cmd/graphedwright render -f web/examples/diamond.json > $(BIN)/native.svg
	python3 -I web/smoke.py go $(BIN)/native.svg
	python3 -I web/smoke.py tinygo $(BIN)/native.svg

# The core packages' tests as wasm under the wazero runtime, so that the
# golden drawings and the editor's behaviour are the same there. wazero
# is a tool, not a dependency: go install github.com/tetratelabs/wazero/cmd/wazero@latest
wasm-test:
	PATH="$$PATH:$$(go env GOPATH)/bin:$$(go env GOROOT)/lib/wasm" GOOS=wasip1 GOARCH=wasm GOWASIRUNTIME=wazero go test -count=1 -timeout 30m . ./svg ./layout ./doc ./edit

# Every package's tests on the other Linux architectures under qemu-user,
# the big-endian and 32-bit ones among them, so that a result that
# depends on the machine fails here. The test binaries run through
# binfmt_misc, which qemu-user-binfmt registers. The time budgets are
# skipped: emulation is several times slower than the hardware.
QEMU_ARCHS := arm64 arm riscv64 ppc64le ppc64 s390x loong64 mips64le mipsle

qemu-test:
	@test -e /proc/sys/fs/binfmt_misc/qemu-aarch64 || { echo "qemu-user with binfmt_misc is required"; exit 1; }
	@failed=; for a in $(QEMU_ARCHS); do \
	  echo "== $$a"; GOARCH=$$a go test -count=1 -timeout 60m -skip 'Budget' $(PKGS) || failed="$$failed $$a"; \
	done; \
	if [ -n "$$failed" ]; then echo "failed on:$$failed"; exit 1; fi

# merilo checks the editor's gesture machine (edit/gesture) and the
# editor that holds it. merilovet, at the version internal/verify
# requires, holds both to merilo's FSM rules, once for each target with
# files of its own: dom builds only for js/wasm. internal/verify has
# Total for the machine, explores every session of host events up to a
# bound against the editor's invariants, replays each bug it found, and
# fuzzes past the bound from the search's witnesses.
MERILO_TARGETS := linux/amd64 js/wasm

merilo:
	cd internal/verify && go mod verify && go mod tidy -diff && go vet ./... \
	  && go build -o $(BIN)/merilovet code.eitanity.com/merilo/cmd/merilovet
	@for t in $(MERILO_TARGETS); do \
	  echo "merilovet $$t"; GOOS=$${t%/*} GOARCH=$${t#*/} $(BIN)/merilovet || exit 1; \
	done
	cd internal/verify && go test -count=1 ./... \
	  && go test -run='^$$' -fuzz='^FuzzEditor$$' -fuzztime=60s .

# Serves the demo page at http://127.0.0.1:8001/ (add ?toolchain=tinygo).
demo: wasm
	go run ./cmd/graphedwright-demo

size: wasm
	@g=$$(stat -c %s $(BIN)/graphedwright.go.wasm); t=$$(stat -c %s $(BIN)/graphedwright.tinygo.wasm); \
	  echo "go $$g bytes, tinygo $$t bytes"; \
	  test $$g -le $(WASM_SIZE_LIMIT_GO) || { echo "go wasm over $(WASM_SIZE_LIMIT_GO)"; exit 1; }; \
	  test $$t -le $(WASM_SIZE_LIMIT_TINY) || { echo "tinygo wasm over $(WASM_SIZE_LIMIT_TINY)"; exit 1; }

# Rewrites the frozen SVGs after an intended change; review the diff.
golden:
	go test -run TestGolden -update $(PKGS)

