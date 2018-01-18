# Copyright 2018 Yunify Inc. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM alpine:3.4

LABEL MAINTAINER="jolestar <jolestar@gmail.com>"

COPY bin/alpine/metad /usr/bin/

EXPOSE 9112
EXPOSE 80

ENTRYPOINT ["/usr/bin/metad"]
