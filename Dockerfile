FROM mischief/docker-golang
RUN apt-get -y install iptables quota rsync net-tools protobuf-compiler
ADD http://cfstacks.s3.amazonaws.com/lucid64.dev.tgz /opt/warden/rootfs
RUN go get code.google.com/p/gogoprotobuf/protoc-gen-gogo
