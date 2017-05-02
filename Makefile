GO?=go

PROG=ws-subprocess
SOURCEDIR=.

SOURCES := $(shell find $(SOURCEDIR) -name '*.go')

dist/$(PROG): $(SOURCES)
	$(GO) build -o $@

.PHONY: clean
clean:
	rm -f $(PROG)
