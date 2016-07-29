GOFLAGS=-installsuffix cgo --ldflags '-w -s' -a
GOBUILDPREFIX=GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO15VENDOREXPERIMENT=1
OUT=/targetdir/wstail

.PHONY: all
all: $(OUT)

$(OUT): $(shell find $$GOPATH/src/github.com/gravitational/logging-app/cmd/wstail -type f)
$(OUT): /assets/Makefile
$(OUT):
	@echo "\n---> Building $@\n"
	$(GOBUILDPREFIX) go build -o $@ $(GOFLAGS) github.com/gravitational/logging-app/cmd/wstail

.PHONY: clean
clean:
	-sudo rm build/*
