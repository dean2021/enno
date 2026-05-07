VERSION := $(shell tr -d '[:space:]' < VERSION)

.PHONY: help fmt tidy test install examples verify version release-check tag

help:
	@echo "Enno development commands:"
	@echo "  make verify        Format, tidy, test, and verify CLI installation"
	@echo "  make fmt           Format Go source files"
	@echo "  make tidy          Tidy Go modules"
	@echo "  make test          Run all Go tests"
	@echo "  make install       Install the local CLI from ./cmd/enno"
	@echo "  make examples      Compile example packages"
	@echo "  make version       Print the current VERSION"
	@echo "  make release-check Validate VERSION, CHANGELOG, tests, and CLI install"
	@echo "  make tag           Create git tag v$$(cat VERSION)"

fmt:
	gofmt -w .

tidy:
	go mod tidy

test:
	go test ./...

install:
	go install ./cmd/enno

examples:
	go test ./examples/...

verify: fmt tidy test install

version:
	@echo $(VERSION)

release-check:
	@test -n "$(VERSION)" || (echo "VERSION is empty" && exit 1)
	@case "$(VERSION)" in v*) echo "VERSION should not include leading v: $(VERSION)" && exit 1 ;; esac
	@grep -Fq "## [$(VERSION)]" CHANGELOG.md || (echo "CHANGELOG.md missing version $(VERSION)" && exit 1)
	@go test ./...
	@go install ./cmd/enno

tag: release-check
	git tag v$(VERSION)
