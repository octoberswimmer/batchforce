SHELL := /bin/bash

VERSION=$(shell git describe --abbrev=0 --always)
LDFLAGS = -ldflags "-s -w -X github.com/octoberswimmer/batchforce.Version=${VERSION}"
EXECUTABLE=batchforce
PACKAGE=./cmd/batchforce
WINDOWS=$(EXECUTABLE)_windows_amd64.exe
WINDOWS_ARM64=$(EXECUTABLE)_windows_arm64.exe
LINUX=$(EXECUTABLE)_linux_amd64
LINUX_ARM64=$(EXECUTABLE)_linux_arm64
DARWIN_AMD64=$(EXECUTABLE)_darwin_amd64
DARWIN_ARM64=$(EXECUTABLE)_darwin_arm64
EMBEDDED=cmd/batchforce/cmd/docs/language-definition.md
ALL=$(WINDOWS) $(WINDOWS_ARM64) $(LINUX) $(LINUX_ARM64) $(DARWIN_AMD64) $(DARWIN_ARM64)
ZIPS=$(addsuffix _$(VERSION).zip,$(basename $(ALL)))
RELEASE_ASSETS=$(ZIPS) SHA256SUMS-$(VERSION)

default: $(EMBEDDED)
	go build -o ${EXECUTABLE} ${LDFLAGS} ${PACKAGE}

install: $(EMBEDDED)
	go install ${LDFLAGS} ${PACKAGE}

$(WINDOWS): $(EMBEDDED)
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -v -o $(WINDOWS) ${LDFLAGS} ${PACKAGE}

$(WINDOWS_ARM64): $(EMBEDDED)
	env CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -v -o $(WINDOWS_ARM64) ${LDFLAGS} ${PACKAGE}

$(LINUX): $(EMBEDDED)
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o $(LINUX) ${LDFLAGS} ${PACKAGE}

$(LINUX_ARM64): $(EMBEDDED)
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -o $(LINUX_ARM64) ${LDFLAGS} ${PACKAGE}

$(DARWIN_AMD64): $(EMBEDDED)
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -v -o $(DARWIN_AMD64) ${LDFLAGS} ${PACKAGE}
	rcodesign sign --for-notarization --pem-file <(pass OctoberSwimmer/codesign/combined) $@

$(DARWIN_ARM64): $(EMBEDDED)
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -v -o $(DARWIN_ARM64) ${LDFLAGS} ${PACKAGE}
	rcodesign sign --for-notarization --pem-file <(pass OctoberSwimmer/codesign/combined) $@

$(basename $(WINDOWS))_$(VERSION).zip: $(WINDOWS)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

$(basename $(WINDOWS_ARM64))_$(VERSION).zip: $(WINDOWS_ARM64)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

$(basename $(DARWIN_AMD64))_$(VERSION).zip: $(DARWIN_AMD64)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)
	rcodesign notary-submit --api-key-file <(pass OctoberSwimmer/codesign/api-key) $@

$(basename $(DARWIN_ARM64))_$(VERSION).zip: $(DARWIN_ARM64)
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)
	rcodesign notary-submit --api-key-file <(pass OctoberSwimmer/codesign/api-key) $@

%_$(VERSION).zip: %
	@rm -f $@
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)

dist: test $(ZIPS)

checksum: dist
	shasum -a 256 $(ZIPS) > SHA256SUMS-$(VERSION)

release: checksum
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "gh CLI is required for 'make release'."; \
		exit 1; \
	fi
	@if ! git rev-parse --verify "refs/tags/$(VERSION)" >/dev/null 2>&1; then \
		echo "Tag '$(VERSION)' does not exist. Create the tag before running 'make release'."; \
		exit 1; \
	fi
	@if [ "$$(git describe --exact-match --tags HEAD 2>/dev/null)" != "$(VERSION)" ]; then \
		echo "HEAD is not exactly at tag '$(VERSION)'. Check out the tag before running 'make release'."; \
		exit 1; \
	fi
	git push octoberswimmer "$(VERSION)"
	gh release create "$(VERSION)" --title "batchforce $(VERSION)" --notes-from-tag --verify-tag $(RELEASE_ASSETS)

# Update embedded Expr language definition
$(EMBEDDED):
	go generate ./cmd/batchforce/cmd

fmt:
	go fmt ./...

test: $(EMBEDDED)
	test -z "$(go fmt)"
	go vet
	go test ./...
	go test -race ./...

docs:
	go run docs/mkdocs.go

clean:
	-rm -f $(EXECUTABLE) $(EXECUTABLE)_* *.zip SHA256SUMS-*

.PHONY: default dist clean docs checksum release fmt test
