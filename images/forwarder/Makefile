override VERSION:=v0.17
GOFLAGS=-installsuffix cgo --ldflags '-w -s' -a
GOBUILDPREFIX=GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO15VENDOREXPERIMENT=1
BASEREPO:=github.com/papertrail
REPO:=$(BASEREPO)/remote_syslog2
BASEDIR:=$(GOPATH)/src/$(BASEREPO)
REPODIR:=$(BASEDIR)/remote_syslog2
OUT=/targetdir/remote_syslog

.PHONY: all
all: $(OUT)

$(OUT): /assets/Makefile
	@echo "\n---> Building $@\n"
	go get github.com/tools/godep
	mkdir -p $(BASEDIR)
	cd $(BASEDIR) && git clone https://$(REPO) -b $(VERSION) --depth=1
	cd $(REPODIR) && $(GOBUILDPREFIX) godep go build $(GOFLAGS) -o $@ .

.PHONY: clean
clean:
	-sudo rm build/*
