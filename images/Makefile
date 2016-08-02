
PWD=$(shell pwd)
REPODIR=$(abspath $(PWD)/../)
export

COLLECTOR_TAG := log-collector:$(VERSION)
FORWARDER_TAG := log-forwarder:$(VERSION)
LOGLINKER_TAG := log-linker:$(VERSION)
CONTAINERS := $(COLLECTOR_TAG) $(FORWARDER_TAG) $(LOGLINKER_TAG)

REPO_URL ?= quay.io/gravitational

.PHONY: all
all: collector forwarder loglinker

.PHONY: collector
collector: collector-build
	docker build --pull -t $(COLLECTOR_TAG) collector

.PHONY: collector-build
collector-build:
	$(MAKE) -C $(PWD)/collector -e TARGET=collector TARGETDIR=collector -f $(PWD)/buildbox.mk

.PHONY: collector-clean
collector-clean:
	$(MAKE) -C $(PWD)/collector clean

.PHONY: forwarder
forwarder: forwarder-build
	docker build --pull -t $(FORWARDER_TAG) forwarder

.PHONY: forwarder-build
forwarder-build:
	$(MAKE) -C $(PWD)/forwarder -e TARGET=forwarder TARGETDIR=forwarder -f $(PWD)/buildbox.mk

.PHONY: forwarder-clean
forwarder-clean:
	$(MAKE) -C $(PWD)/forwarder clean

.PHONY: loglinker
loglinker: loglinker-build
	docker build --pull -t $(LOGLINKER_TAG) loglinker

.PHONY: loglinker-build
loglinker-build:
	$(MAKE) -C $(PWD)/loglinker -e TARGET=loglinker TARGETDIR=loglinker -f $(PWD)/buildbox.mk

.PHONY: loglinker-clean
loglinkerer-clean:
	$(MAKE) -C $(PWD)/loglinker clean

.PHONY: deploy
deploy:
	$(foreach ct,$(CONTAINERS), \
		docker tag $(ct) $(REPO_URL)/$(ct) ; \
		docker push $(REPO_URL)/$(ct) ; )

.PHONY: clean
clean: collector-clean forwarder-clean loglinker-clean
