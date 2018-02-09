#!/usr/bin/dumb-init /bin/sh

tail_port=:$1

if [ "$tail_port" = ":" ]; then
	tail_port=:8083
fi

opts=
if [ "x$DEBUG" != "x" ]; then
  export RSYSLOG_DEBUG=Debug
  export RSYSLOG_DEBUGLOG=/var/log/rsyslog.log
  opts=-debug
fi

/usr/sbin/rsyslogd
/wstail -addr=$tail_port $opts
