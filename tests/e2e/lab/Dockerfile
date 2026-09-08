FROM alpine:3.22

RUN apk add --no-cache \
    bash \
    bind-tools \
    dnsmasq \
    iproute2 \
    iputils \
    nftables \
    wireguard-tools

COPY run.sh /usr/local/bin/run-lab
RUN chmod 0755 /usr/local/bin/run-lab

ENTRYPOINT ["/usr/local/bin/run-lab"]
