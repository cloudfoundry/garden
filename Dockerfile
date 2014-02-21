# this builds the base image for Garden's CI

FROM mischief/docker-golang

# pull in dependencies for the server
RUN apt-get -y install iptables quota rsync net-tools protobuf-compiler

# pull in the prebuilt rootfs
ADD garden-test-rootfs.tar /opt/warden/rootfs

# install the binary for generating the protocol
RUN go get code.google.com/p/gogoprotobuf/protoc-gen-gogo
