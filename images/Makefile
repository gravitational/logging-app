VER := 0.0.2

COLLECTOR_TAG := log-collector:$(VER)
FORWARDER_TAG := log-forwarder:$(VER)

REPO_URL ?= quay.io/gravitational

.PHONY: all
all: collector forwarder

.PHONY: collector
collector:
	docker build --pull -t $(COLLECTOR_TAG) collector

# TODO: remote_syslog2 binary is in repo, make it build in Docker ct
.PHONY: forwarder
forwarder:
	docker build --pull -t $(FORWARDER_TAG) forwarder

.PHONY: deploy
deploy:
	$(foreach ct,$(COLLECTOR_TAG) $(FORWARDER_TAG), \
		docker tag $(ct) $(REPO_URL)/$(ct) ; \
		docker push $(REPO_URL)/$(ct) ; )

