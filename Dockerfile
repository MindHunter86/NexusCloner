# -*- coding: utf-8 -*-
# vim: ft=Dockerfile

FROM golang:1.16-alpine AS build
LABEL maintainer="vkom <admin@vkom.cc>"

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
COPY cloner ./
RUN go build -ldflags="-s -w" -o /NexusCloner

RUN apk add --no-cache upx \
  && upx -9 -k /NexusCloner \
  && apk del upx


FROM alpine
LABEL maintainer="vkom <admin@vkom.cc>"

WORKDIR /

COPY --from=build /NexusCloner /usr/local/bin

USER nobody
ENTRYPOINT ["/usr/local/bin/NexusCloner"]
