FROM gliderlabs/alpine:latest
MAINTAINER Chris Snell <chris.snell@gmail.com>
ADD sparkyfish-server-linux-amd64 /sparkyfish-server
ENV DEBUG false
ENV CNAME none
ENV LOCATION none
ENV LISTEN_ADDR :7121
EXPOSE 7121

CMD /sparkyfish-server -listen-addr=$LISTEN_ADDR -debug=$DEBUG -cname=$CNAME -location=$LOCATION
