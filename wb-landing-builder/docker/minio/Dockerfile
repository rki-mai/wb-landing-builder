FROM minio/mc:latest AS mc
FROM minio/minio:latest

COPY --from=mc /usr/bin/mc /usr/bin/mc
COPY scripts/ready.sh /usr/local/bin/minio-ready.sh
COPY scripts/healthcheck.sh /usr/local/bin/minio-healthcheck.sh
COPY lib/common.sh /usr/local/lib/minio/common.sh
COPY entrypoint.sh /usr/local/bin/minio-entrypoint.sh

RUN chmod +x /usr/local/bin/minio-ready.sh \
    /usr/local/bin/minio-healthcheck.sh \
    /usr/local/bin/minio-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/minio-entrypoint.sh"]
CMD ["server", "/data", "--console-address", ":9001"]
