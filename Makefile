all: protocol

protocol: $(shell find protobuf/ -type f)
	mkdir -p protocol/
	rm -f protocol/*.pb.go
	protoc --gogo_out=protocol/ --proto_path=protobuf/ protobuf/*.proto

.PHONY: protocol
