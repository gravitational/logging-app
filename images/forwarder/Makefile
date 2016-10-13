# We need a version >v0.18 as it contains
# https://github.com/papertrail/remote_syslog2/pull/174
# in which remote_syslog2 forwards new files entirely
override VERSION:=9151f7e8eab2d69666aa43e6aeb56fc3aa4065fc
GOFLAGS=-installsuffix cgo --ldflags '-w -s' -a
GOBUILDPREFIX=GOOS=linux GOARCH=amd64 CGO_ENABLED=0
BASEREPO:=github.com/papertrail
REPO:=$(BASEREPO)/remote_syslog2
BASEDIR:=$(GOPATH)/src/$(BASEREPO)
REPODIR:=$(BASEDIR)/remote_syslog2
OUT=/targetdir/remote_syslog
GOLDFLAGS="-X main.Version=$(VERSION)"

.PHONY: all
all: $(OUT)

$(OUT): /assets/Makefile
	@echo "\n---> Building $@\n"
	mkdir -p $(BASEDIR)
	cd $(BASEDIR) && git clone https://$(REPO)
	cd $(REPODIR) && git checkout $(VERSION)
	cd $(REPODIR) && $(GOBUILDPREFIX) go build $(GOFLAGS) -ldflags=$(GOLDFLAGS) -o $@ .

.PHONY: clean
clean:
	-sudo rm build/*
