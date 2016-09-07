FROM alpine:3.4
MAINTAINER jolestar <jolestar@gmail.com>

COPY bin/metad /usr/bin

EXPOSE 9112
EXPOSE 80

CMD ["metad"]
