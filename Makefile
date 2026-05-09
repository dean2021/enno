VERSION := $(shell tr -d '[:space:]' < VERSION)

.PHONY: help fmt tidy test examples verify version release-check tag

help:
	@echo "Enno SDK development commands:"
	@echo "  make verify        Format, tidy, and test the SDK module"
	@echo "  make fmt           Format Go source files"
	@echo "  make tidy          Tidy Go modules"
	@echo "  make test          Run all Go tests"
	@echo "  make examples      Compile example packages"
	@echo "  make version       Print the current VERSION"
	@echo "  make release-check Validate VERSION, CHANGELOG, and tests"
	@echo "  make tag           Create git tag v$$(cat VERSION)"

fmt:
	gofmt -w .

tidy:
	go mod tidy

test:
	go test ./...

examples:
	go test ./examples/...

verify: fmt tidy test

version:
	@echo $(VERSION)

release-check:
	@test -n "$(VERSION)" || (echo "VERSION is empty" && exit 1)
	@case "$(VERSION)" in v*) echo "VERSION should not include leading v: $(VERSION)" && exit 1 ;; esac
	@grep -Fq "## [$(VERSION)]" CHANGELOG.md || (echo "CHANGELOG.md missing version $(VERSION)" && exit 1)
	@go test ./...

tag: release-check
	git tag v$(VERSION)
