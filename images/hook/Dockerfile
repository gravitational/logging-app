FROM quay.io/gravitational/rig:7.1.3

ARG CHANGESET
ENV RIG_CHANGESET $CHANGESET

ADD entrypoint.sh /

ENTRYPOINT ["dumb-init", "/entrypoint.sh"]
