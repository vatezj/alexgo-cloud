package audit

import "context"

type OperateRecorder interface {
	RecordOperate(ctx context.Context, userID uint64, username string, method string, path string, status int, latencyMs int64, errMsg string) error
}

