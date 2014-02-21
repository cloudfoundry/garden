all: protocol skeleton

protocol: $(shell find protobuf/ -type f)
	mkdir -p protocol/
	rm protocol/*.pb.go
	protoc --gogo_out=protocol/ --proto_path=protobuf/ protobuf/*.proto

skeleton:
	cd linux_backend/src && make clean all
	cp linux_backend/src/wsh/wshd linux_backend/skeleton/bin
	cp linux_backend/src/wsh/wsh linux_backend/skeleton/bin
	cp linux_backend/src/oom/oom linux_backend/skeleton/bin
	cp linux_backend/src/iomux/iomux-spawn linux_backend/skeleton/bin
	cp linux_backend/src/iomux/iomux-link linux_backend/skeleton/bin
	cp linux_backend/src/repquota/repquota linux_backend/bin

garden-test-rootfs.cid: integration/rootfs/Dockerfile
	docker build -t cloudfoundry/garden-test-rootfs --rm integration/rootfs
	docker run -cidfile=garden-test-rootfs.cid cloudfoundry/garden-test-rootfs echo

garden-test-rootfs.tar: garden-test-rootfs.cid
	docker export `cat garden-test-rootfs.cid` > garden-test-rootfs.tar
	docker rm `cat garden-test-rootfs.cid`
	rm garden-test-rootfs.cid

ci-image: garden-test-rootfs.tar
	docker build -t cloudfoundry/garden-ci --rm .
	rm garden-test-rootfs.tar
