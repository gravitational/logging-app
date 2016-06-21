VER := 0.0.1
BOOTSTRAP_TAG := gravitational.io/log-bootstrap:$(VER)
UNINSTALL_TAG := gravitational.io/log-uninstall:$(VER)
COLLECTOR_TAG := gravitational.io/log-collector:$(VER)
FORWARDER_TAG := gravitational.io/log-forwarder:$(VER)

.PHONY: all
all: bootstrap uninstall collector forwarder

.PHONY: bootstrap
bootstrap:
	docker build -t $(BOOTSTRAP_TAG) bootstrap

.PHONY: uninstall
uninstall:
	docker build -t $(UNINSTALL_TAG) uninstall

.PHONY: collector
collector:
	docker build -t $(COLLECTOR_TAG) collector

# TODO: remote_syslog2 binary is in repo, make it build in Docker ct
.PHONY: forwarder
forwarder:
	docker build -t $(FORWARDER_TAG) forwarder
