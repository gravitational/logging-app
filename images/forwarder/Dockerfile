FROM debian:jessie

ENV DEBIAN_FRONTEND noninteractive

ADD locale.gen /etc/locale.gen
ADD profile /etc/profile

RUN (apt-get clean \
    && apt-key update \
    && apt-get -q -y update --fix-missing \
    && apt-get -q -y update \
    && apt-get install -q -y apt-utils \
    && apt-get install -q -y less \
    && apt-get install -q -y locales) ;

# Set locale to en_US.UTF-8
RUN (locale-gen \
    && locale-gen en_US.UTF-8 \
    && dpkg-reconfigure locales)

ADD ./remote_syslog /usr/local/bin/remote_syslog
ADD ./remote_syslog.yml /etc/remote_syslog.yml

RUN chmod +x /usr/local/bin/remote_syslog

ENTRYPOINT ["/usr/local/bin/remote_syslog", "--poll", "-D", "-c", "/etc/remote_syslog.yml"]
