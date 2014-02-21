# this builds the rootfs that will be used for the ingration tests
#
# see Makefile

FROM cloudfoundry/lucid64:minbase

RUN apt-get -y install ruby rsync

ADD seed /etc/seed
