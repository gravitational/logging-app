FROM quay.io/gravitational/debian-grande:0.0.1

ADD ./build/remote_syslog /usr/local/bin/remote_syslog
ADD ./remote_syslog.yml /etc/remote_syslog.yml

RUN chmod +x /usr/local/bin/remote_syslog

RUN test -f /cleanup.sh && sh /cleanup.sh

ENTRYPOINT ["/usr/local/bin/remote_syslog", "--poll", "-D", "--reopen=false", "-c", "/etc/remote_syslog.yml"]
