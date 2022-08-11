package trace

import (
	"context"
	"encoding/binary"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

var _hostHash byte

func init() {
	rand.Seed(time.Now().UnixNano())
	Hostname, err := os.Hostname()
	if err != nil {
		Hostname = strconv.Itoa(int(time.Now().UnixNano()))
	}
	_hostHash = byte(oneAtTimeHash(Hostname))
}

func oneAtTimeHash(s string) (hash uint32) {
	b := []byte(s)
	for i := range b {
		hash += uint32(b[i])
		hash += hash << 10
		hash ^= hash >> 6
	}
	hash += hash << 3
	hash ^= hash >> 11
	hash += hash << 15
	return
}

func genID() uint64 {
	var b [8]byte
	// 我认为这段代码将无法生存到2106-02-07
	binary.BigEndian.PutUint32(b[4:], uint32(time.Now().Unix())>>8)
	b[4] = _hostHash
	binary.BigEndian.PutUint32(b[:4], uint32(rand.Int31()))
	return binary.BigEndian.Uint64(b[:])
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

type ctxKey string

var _ctxKey ctxKey = "fos/trace.trace"

// FromContext 返回绑定到上下文的跟踪(如果有).
func FromContext(ctx context.Context) (t Trace, ok bool) {
	t, ok = ctx.Value(_ctxKey).(Trace)
	return
}

// NewContext 新的跟踪上下文.注意:此方法不是线程安全的.
func NewContext(ctx context.Context, t Trace) context.Context {
	return context.WithValue(ctx, _ctxKey, t)
}
