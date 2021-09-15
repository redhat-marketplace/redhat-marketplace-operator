package server

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ConvertTimestamp(t int64) (out *timestamppb.Timestamp) {
	if t != 0 {
		out, _ = ptypes.TimestampProto(time.Unix(t, 0))
		return
	}

	return
}
