FROM mongo:7

COPY scripts/ping.sh /usr/local/bin/mongo-ping.sh
COPY scripts/primary.sh /usr/local/bin/mongo-primary.sh
COPY scripts/healthcheck.sh /usr/local/bin/mongo-healthcheck.sh
COPY lib/common.sh /usr/local/lib/mongo/common.sh
COPY entrypoint.sh /usr/local/bin/mongo-entrypoint.sh

RUN chmod +x /usr/local/bin/mongo-ping.sh \
    /usr/local/bin/mongo-primary.sh \
    /usr/local/bin/mongo-healthcheck.sh \
    /usr/local/bin/mongo-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/mongo-entrypoint.sh"]
