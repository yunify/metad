# Copyright 2018 Yunify Inc. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.7.3-alpine

ENV GOPATH /go

RUN mkdir -p "$GOPATH/src/" "$GOPATH/bin" && chmod -R 777 "$GOPATH" && \
    mkdir -p /go/src/github.com/yunify/metad

RUN apk --update add bash git && \
    ln -s /go/src/github.com/yunify/metad /app

WORKDIR /app
