GOFLAGS=-installsuffix cgo --ldflags '-w -s' -a
GOBUILDPREFIX=GOOS=linux GOARCH=amd64 CGO_ENABLED=0
OUT=/targetdir/adapter

.PHONY: all
all: $(OUT)

$(OUT): $(shell find $$GOPATH/src/github.com/gravitational/logging-app/cmd/adapter -type f)
$(OUT): /assets/Makefile
$(OUT):
	@echo "\n---> Building $@\n"
	$(GOBUILDPREFIX) go build -o $@ $(GOFLAGS) github.com/gravitational/logging-app/cmd/adapter

.PHONY: clean
clean:
	-sudo rm build/*
