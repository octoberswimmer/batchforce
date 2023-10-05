VERSION=$(shell git describe --abbrev=0 --always)
LDFLAGS = -ldflags "-s -w -X github.com/octoberswimmer/batchforce.Version=${VERSION}"
EXECUTABLE=batchforce
PACKAGE=./cmd/batchforce
WINDOWS=$(EXECUTABLE)-windows-amd64.exe
LINUX=$(EXECUTABLE)-linux-amd64
OSX_AMD64=$(EXECUTABLE)-darwin-amd64
OSX_ARM64=$(EXECUTABLE)-darwin-arm64
ALL=$(WINDOWS) $(LINUX) $(OSX_ARM64) $(OSX_AMD64)

default:
	go build -o ${EXECUTABLE} ${LDFLAGS} ${PACKAGE}

install:
	go install ${LDFLAGS} ${PACKAGE}

#$(WINDOWS): checkcmd-x86_64-w64-mingw32-gcc checkcmd-x86_64-w64-mingw32-g++
#	env \
#		GOOS=windows \
#		GOARCH=amd64 \
#		CC=x86_64-w64-mingw32-gcc \
#		CXX=x86_64-w64-mingw32-g++ \
#		CGO_ENABLED=1 \
#		CGO_CFLAGS=-D_WIN32_WINNT=0x0400 \
#		CGO_CXXFLAGS=-D_WIN32_WINNT=0x0400 \
#		go build -v -o $(WINDOWS) ${LDFLAGS} ${PACKAGE}

$(WINDOWS): checkcmd-xgo
	xgo -go 1.21 -out $(EXECUTABLE) -dest . ${LDFLAGS} -buildmode default -trimpath -targets windows/amd64 -pkg ${PACKAGE} -x .

$(LINUX): checkcmd-x86_64-linux-gnu-gcc checkcmd-x86_64-linux-gnu-g++
	env \
		GOOS=linux \
		GOARCH=amd64 \
		CC=x86_64-linux-gnu-gcc \
		CXX=x86_64-linux-gnu-g++ \
		CGO_ENABLED=1 \
		go build -v -o $(LINUX) ${LDFLAGS} ${PACKAGE}

# Build macOS binaries using docker images that contain SDK
# See https://github.com/crazy-max/xgo and https://github.com/tpoechtrager/osxcross
$(OSX_ARM64): checkcmd-xgo
	xgo -go 1.21 -out $(EXECUTABLE) -dest . ${LDFLAGS} -buildmode default -trimpath -targets darwin/arm64 -pkg ${PACKAGE} -x .

$(OSX_AMD64): checkcmd-xgo
	xgo -go 1.21 -out $(EXECUTABLE) -dest . ${LDFLAGS} -buildmode default -trimpath -targets darwin/amd64 -pkg ${PACKAGE} -x .

$(basename $(WINDOWS)).zip: $(WINDOWS)
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)$(suffix $<)

%.zip: %
	zip $@ $<
	7za rn $@ $< $(EXECUTABLE)

dist: test $(addsuffix .zip,$(basename $(ALL)))

test:
	test -z "$(go fmt)"
	go vet
	go test ./...
	go test -race ./...

docs:
	go run docs/mkdocs.go

checkcmd-%:
	@hash $(*) > /dev/null 2>&1 || \
		(echo "ERROR: '$(*)' must be installed and available on your PATH."; exit 1)

clean:
	-rm -f $(EXECUTABLE) $(EXECUTABLE)_*

.PHONY: default dist clean docs
