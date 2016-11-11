FROM alpine:3.4
MAINTAINER jolestar <jolestar@gmail.com>

COPY bin/alpine/metad /usr/bin/

EXPOSE 9112
EXPOSE 80

ENTRYPOINT ["/usr/bin/metad"]
