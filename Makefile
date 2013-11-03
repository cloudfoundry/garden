all:
	mkdir -p protocol/
	protoc --gogo_out=protocol/ --proto_path=protobuf/ protobuf/*.proto
