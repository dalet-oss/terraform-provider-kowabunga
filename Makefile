BINDIR = bin
BIN = terraform-provider-kowabunga
LDFLAGS += -X main.version=$$(git describe --always --abbrev=40 --dirty)

GOVULNCHECK = $(BINDIR)/govulncheck
GOVULNCHECK_VERSION = v1.0.4

GOLINT = $(BINDIR)/golangci-lint
GOLINT_VERSION = v1.56.2

V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

.PHONY: all
all: mod fmt lint vet $(BIN) ; @

.PHONY: mod
mod: ; $(info $(M) collecting modules…) @
	$Q go mod download
	$Q go mod tidy

.PHONY: fmt
fmt: ; $(info $(M) formatting code…) @
	$Q go fmt ./internal/provider .

.PHONY: get-lint
get-lint: ; $(info $(M) downloading go-lint…) @
	$Q test -x $(GOLINT) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s $(GOLINT_VERSION)

.PHONY: lint
lint: get-lint ; $(info $(M) running linter…) @
	$Q $(GOLINT) run -e SA1019 ./... ; exit 0

.PHONY: get-govulncheck
get-govulncheck: ; $(info $(M) downloading govulncheck…) @
	$Q test -x $(GOVULNCHECK) || GOBIN="$(PWD)/$(BINDIR)/" go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

.PHONY: vuln
vuln: get-govulncheck ; $(info $(M) running govulncheck…) @ ## Check for known vulnerabilities
	$Q $(GOVULNCHECK) ./... ; exit 0

.PHONY: vet
vet: ; $(info $(M) running vetter…) @
	$Q go vet ./internal/provider .

.PHONY: doc
doc: ; $(info $(M) generating documentation…) @
	$Q go generate ./...

.PHONY: $(BIN)
$(BIN): ; $(info $(M) building terraform provider plugin…) @
	$Q go build -ldflags "${LDFLAGS}"

.PHONY: install
install: ; $(info $(M) installing terraform provider plugin…) @
	$Q go install -ldflags "${LDFLAGS}"

.PHONY: clean
clean: ; $(info $(M) cleanup…) @
	$Q rm -f $(BIN)
	$Q rm -f $(DINDIR)
