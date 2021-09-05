// +build tools

package tools

import (
	_ "github.com/twitchtv/twirp/protoc-gen-twirp"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "storj.io/drpc/cmd/protoc-gen-go-drpc"
)
