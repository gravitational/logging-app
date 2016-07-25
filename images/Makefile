
PWD=$(shell pwd)
REPODIR=$(abspath $(PWD)/../)
export

TAILER_TAG := log-tailer:$(VERSION)
COLLECTOR_TAG := log-collector:$(VERSION)
FORWARDER_TAG := log-forwarder:$(VERSION)
CONTAINERS := $(COLLECTOR_TAG) $(FORWARDER_TAG) $(TAILER_TAG)

REPO_URL ?= quay.io/gravitational

.PHONY: all
all: collector forwarder tailer

.PHONY: collector
collector:
	docker build --pull -t $(COLLECTOR_TAG) collector

# TODO: remote_syslog2 binary is in repo, make it build in Docker ct
.PHONY: forwarder
forwarder:
	docker build --pull -t $(FORWARDER_TAG) forwarder

.PHONY: tailer
tailer: tailer-build
	docker build --pull -t $(TAILER_TAG) tailer
	docker tag -f $(TAILER_TAG) $(REPO_URL)/$(TAILER_TAG)

.PHONY: tailer-build
tailer-build:
	$(MAKE) -C $(PWD)/tailer -e TARGET=tailer TARGETDIR=tailer -f $(PWD)/buildbox.mk

.PHONY: tailer-clean
tailer-clean:
	$(MAKE) -C $(PWD)/tailer clean

.PHONY: deploy
deploy:
	$(foreach ct,$(CONTAINERS), \
		docker tag $(ct) $(REPO_URL)/$(ct) ; \
		docker push $(REPO_URL)/$(ct) ; )

.PHONY: clean
clean: tailer-clean
