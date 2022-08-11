package trace

import (
	"math/rand"
	"sync/atomic"
	"time"
)

const slotLength = 2048

var ignores = []string{"/metrics", "/healthy", "/grpc.health.v1.Health/Check"} // NOTE: add YOUR URL PATH that want ignore

// sampler decides whether a new trace should be sampled or not.
type sampler interface {
	IsSampled(traceID uint64, operationName string) (bool, float32)
	Close() error
}

type probabilitySampling struct {
	probability float32
	slot        [slotLength]int64
}

func (p *probabilitySampling) IsSampled(traceID uint64, operationName string) (bool, float32) {
	for _, ignored := range ignores {
		if operationName == ignored {
			return false, 0
		}
	}
	now := time.Now().Unix()
	idx := oneAtTimeHash(operationName) % slotLength
	old := atomic.LoadInt64(&p.slot[idx])
	if old != now {
		atomic.SwapInt64(&p.slot[idx], now)
		return true, 1
	}
	return rand.Float32() < p.probability, p.probability
}

func (p *probabilitySampling) Close() error { return nil }

// newSampler new probability sampler
func newSampler(probability float32) sampler {
	if probability <= 0 || probability > 1 {
		panic("probability P ∈ (0, 1]")
	}
	return &probabilitySampling{probability: probability}
}
