FROM quay.io/gravitational/debian-grande:0.0.1

RUN (apt-key update \
    && apt-get -q -y update \
    && apt-get install -q -y --no-install-recommends rsyslog dumb-init) ;

ADD ./rsyslog.conf /etc/rsyslog.conf
RUN chmod 0644 /etc/rsyslog.conf

ADD ./log_rotation_script /root/log_rotation_script
RUN chmod 0755 /root/log_rotation_script

ADD Dockerfile /
ADD entrypoint.sh /
ADD build/wstail /

RUN test -f /cleanup.sh && sh /cleanup.sh

ENTRYPOINT ["dumb-init", "/entrypoint.sh"]
