package store

import (
	"sync/atomic"
	"time"
)

var seq atomic.Int64

func nowNano() int64 {
	n := time.Now().UnixNano()
	return n + seq.Add(1)
}
