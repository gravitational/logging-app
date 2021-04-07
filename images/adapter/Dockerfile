FROM quay.io/gravitational/debian-tall:buster

RUN mkdir -p /opt/logrange/gravity

COPY ["build/adapter", "/opt/logrange/gravity/"]

ENTRYPOINT ["/usr/bin/dumb-init", "/opt/logrange/gravity/adapter", "start", \
"--config-file", "/opt/logrange/gravity/config/adapter.json"]
