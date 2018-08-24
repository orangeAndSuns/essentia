FROM alpine:3.7

RUN \
  apk add --update go git make gcc musl-dev linux-headers ca-certificates && \
  git clone --depth 1 https://github.com/orangeAndSuns/essentia && \
  (cd essentia && make gess) && \
  cp essentia/build/bin/gess /gess && \
  apk del go git make gcc musl-dev linux-headers && \
  rm -rf /essentia && rm -rf /var/cache/apk/*

EXPOSE 8545
EXPOSE 30303

ENTRYPOINT ["/gess"]
