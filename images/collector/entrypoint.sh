#!/usr/bin/dumb-init /bin/sh

tail_port=:$1

if [ "$tail_port" = ":" ]; then
	tail_port=:8083
fi

/usr/sbin/rsyslogd
/wstail -addr=$tail_port
