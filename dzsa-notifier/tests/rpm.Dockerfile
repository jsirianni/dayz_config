FROM almalinux:9
ENV container=docker
RUN dnf install -y systemd procps-ng util-linux curl-minimal ca-certificates && dnf clean all
CMD ["/sbin/init"]
