GOFLAGS=-installsuffix cgo --ldflags '-w -s -extldflags "-static"' -a
GOBUILDPREFIX=GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO15VENDOREXPERIMENT=1
OUT=/targetdir/loglink

.PHONY: all
all: $(OUT)

$(OUT): $(shell find $$GOPATH/src/github.com/gravitational/logging-app/cmd/loglink -type f)
$(OUT): /assets/Makefile
$(OUT):
	@echo "\n---> Building $@\n"
	$(GOBUILDPREFIX) go build -o $@ $(GOFLAGS) github.com/gravitational/logging-app/cmd/loglink

.PHONY: clean
clean:
	-sudo rm build/*
