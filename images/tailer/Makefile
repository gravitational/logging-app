REPO:=https://github.com/gravitational/wstail.git
BASEDIR:=$(GOPATH)/src/github.com/gravitational
REPODIR:=$(BASEDIR)/wstail
VER=dmitri/wstail # FIXME
GOFLAGS=-installsuffix cgo --ldflags '-w -s'
GOBUILDPREFIX=GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO15VENDOREXPERIMENT=1
OUT=/targetdir/wstail

.PHONY: all
all: $(OUT)

$(OUT):
	@echo "\n---> Building $@\n"
	go get github.com/tools/godep
	mkdir -p $(BASEDIR)
	cd $(BASEDIR) && git clone $(REPO) -b $(VER)
	cd $(REPODIR) && $(GOBUILDPREFIX) godep go build -o /targetdir/wstail $(GOFLAGS) ./tool/...

.PHONY: clean
clean:
	sudo rm build/*
