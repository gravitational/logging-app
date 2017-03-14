override VERSION:=f55ba1b5cae7aaf36416f65ce0f1b461a1f4b95e
GOFLAGS=-installsuffix cgo --ldflags '-w -s' -a
GOBUILDPREFIX=GOOS=linux GOARCH=amd64 CGO_ENABLED=0
BASEREPO:=github.com/gravitational
ORIGREPO:=github.com/papertrail
REPO:=$(BASEREPO)/remote_syslog2
BASEDIR:=$(GOPATH)/src/$(ORIGREPO)
REPODIR:=$(BASEDIR)/remote_syslog2
OUT=/targetdir/remote_syslog
GOLDFLAGS="-X main.Version=$(VERSION)"

.PHONY: all
all: $(OUT)

$(OUT): /assets/Makefile
	@echo "\n---> Building $@\n"
	mkdir -p $(BASEDIR)
	cd $(BASEDIR) && git clone https://$(REPO)
	cd $(REPODIR) && git checkout $(VERSION) && \
		$(GOBUILDPREFIX) go build $(GOFLAGS) -ldflags=$(GOLDFLAGS) -o $@ .

.PHONY: clean
clean:
	-sudo rm build/*
