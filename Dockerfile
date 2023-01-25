
##############
# Build stage
FROM techknowlogick/xgo:go-1.19.2 AS build-env

ENV TARGETOS=linux
ENV TARGETARCH=amd64
ENV TARGETVARIANT=v

RUN \
  go install github.com/magefile/mage@latest && \
  mv /go/bin/mage /usr/local/go/bin
  # mv /go/bin/mage /go/src/code.vikunja.io/api/checklist_vikunja

# ARG VIKUNJA_VERSION

# Setup repo
COPY . /go/src/code.vikunja.io/api
WORKDIR /go/src/code.vikunja.io/api

# ARG TARGETOS TARGETARCH TARGETVARIANT
# Checkout version if set

RUN git clone https://diamonddev107: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@github.com/diamonddev107/checklist_vikunja
# WORKDIR /go/src/code.vikunja.io/api/checklist_vikunja
# RUN pwd
RUN mage build:clean
# COPY go.mod /dist/binaries
RUN mage release:xgo linux/*
# RUN /usr/bin/xgo -dest /go/src/code.vikunja.io/api/checklist_vikunja/dist/binaries -tags netgo  -ldflags -linkmode external -extldflags "-static" -targets linux/* -out vikunja-unstable /go/src/code.vikunja.io/api/checklist_vikunja

# WORKDIR /go/src/code.vikunja.io/api/

###################
# The actual image
# Note: I wanted to use the scratch image here, but unfortunatly the go-sqlite bindings require cgo and
# because of this, the container would not start when I compiled the image without cgo.
FROM alpine:3.16
LABEL maintainer="maintainers@vikunja.io"

WORKDIR /app/vikunja/
COPY --from=build-env /build/vikunja-* vikunja
ENV VIKUNJA_SERVICE_ROOTPATH=/app/vikunja/

# Dynamic permission changing stuff
ENV PUID 1000
ENV PGID 1000
RUN apk --no-cache add shadow && \
  addgroup -g ${PGID} vikunja && \
  adduser -s /bin/sh -D -G vikunja -u ${PUID} vikunja -h /app/vikunja -H && \
  chown vikunja -R /app/vikunja
COPY run.sh /run.sh

# Add time zone data
RUN apk --no-cache add tzdata

# Files permissions
RUN mkdir /app/vikunja/files && \
  chown -R vikunja /app/vikunja/files
VOLUME /app/vikunja/files

CMD ["/run.sh"]
EXPOSE 3456
