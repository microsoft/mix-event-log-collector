FROM golang:1.22-alpine AS builder

RUN apk update \
    && apk add --no-cache \
    build-base \
    git \
    bash

WORKDIR /nuance

COPY go.mod go.sum main.go scripts/setup.sh ./
COPY pkg pkg
COPY cmd cmd
RUN mkdir -p configs

RUN chmod +x setup.sh \
    && ./setup.sh \
    && rm setup.sh

# final stage
FROM alpine:3
WORKDIR /nuance
RUN adduser -D elc && chown -R elc /nuance
COPY --chown=root:root --from=builder /nuance/bin ./bin
COPY --chown=root:root --from=builder /nuance/configs ./configs
EXPOSE 8078
USER elc

HEALTHCHECK --interval=5m --timeout=3s \
  CMD curl -f http://localhost:8078/ping || exit 1

ENTRYPOINT [ "./bin/event-log-client" ]
