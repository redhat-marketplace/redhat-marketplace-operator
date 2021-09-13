package server

//go:generate protoc -I. -I../bin/include --go_out=paths=source_relative:. model/model.proto
//go:generate protoc -I. -I../bin/include --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. --go-drpc_out=paths=source_relative:. --twirp_out=paths=source_relative:. fileretriever/fileretriever.proto
//go:generate protoc -I. -I../bin/include --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. --go-drpc_out=paths=source_relative:. --twirp_out=paths=source_relative:. filesender/filesender.proto
//go:generate protoc -I. -I../bin/include --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. --go-drpc_out=paths=source_relative:. --twirp_out=paths=source_relative:. adminserver/adminserver.proto
