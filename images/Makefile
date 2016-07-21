
PWD=$(shell pwd)

TAILER_TAG := log-tailer:$(VERSION)
COLLECTOR_TAG := log-collector:$(VERSION)
FORWARDER_TAG := log-forwarder:$(VERSION)

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

.PHONY: tailer-build
tailer-build:
	$(MAKE) -C $(PWD)/tailer -e TARGET=tailer TARGETDIR=tailer -f $(PWD)/buildbox.mk

.PHONY: tailer-clean
tailer-clean:
	$(MAKE) -C $(PWD)/tailer clean

.PHONY: deploy
deploy:
	$(foreach ct,$(COLLECTOR_TAG) $(FORWARDER_TAG), \
		docker tag $(ct) $(REPO_URL)/$(ct) ; \
		docker push $(REPO_URL)/$(ct) ; )

.PHONY: clean
clean: tailer-clean
