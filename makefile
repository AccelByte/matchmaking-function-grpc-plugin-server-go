proto:
	protoc --proto_path=pkg/proto --go_out=pkg/pb \
	--go_opt=paths=source_relative --go-grpc_out=pkg/pb \
	--go-grpc_opt=paths=source_relative pkg/proto/*.proto
