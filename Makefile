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
	cp linux_backend/src/repquota/repquota linux_backend/libexec
