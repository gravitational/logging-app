FROM debian:jessie

ENV DEBIAN_FRONTEND noninteractive

ADD locale.gen /etc/locale.gen
ADD profile /etc/profile

RUN (apt-get clean \
    && apt-key update \
    && apt-get -q -y update --fix-missing \
    && apt-get -q -y update \
    && apt-get install -q -y apt-utils less locales rsyslog \
    && apt-get install -q -y less \
    && apt-get install -q -y locales) ;

# Set locale to en_US.UTF-8
RUN (locale-gen \
    && locale-gen en_US.UTF-8 \
    && dpkg-reconfigure locales)

ADD ./rsyslog.conf /etc/rsyslog.conf
RUN chmod 0644 /etc/rsyslog.conf

ADD ./log_rotation_script /root/log_rotation_script
RUN chmod 0755 /root/log_rotation_script

ENTRYPOINT ["rsyslogd", "-f", "/etc/rsyslog.conf", "-n"]
