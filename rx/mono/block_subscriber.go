package mono

import (
	"context"

	"github.com/jjeffcaii/reactor-go"
	"github.com/rsocket/rsocket-go/internal/common"
	"github.com/rsocket/rsocket-go/payload"
	"go.uber.org/atomic"
)

type blockSubscriber struct {
	// Atomic bool to ensure that 'done' is closed only once.
	isDone *atomic.Bool
	done   chan struct{}
	vchan  chan<- payload.Payload
	echan  chan<- error
}

func newBlockSubscriber(
	done chan struct{},
	vchan chan<- payload.Payload,
	echan chan<- error,
) reactor.Subscriber {
	return blockSubscriber{
		isDone: atomic.NewBool(false),
		done:   done,
		vchan:  vchan,
		echan:  echan,
	}
}

func (b blockSubscriber) OnComplete() {
	swapped := b.isDone.CAS(false, true)
	if swapped {
		close(b.done)
	}
}

func (b blockSubscriber) OnError(err error) {
	swapped := b.isDone.CAS(false, true)
	if swapped {
		b.echan <- err
		close(b.done)
	}
}

func (b blockSubscriber) OnNext(any reactor.Any) {
	if !b.isDone.Load() {
		if r, ok := any.(common.Releasable); ok {
			r.IncRef()
		}
		b.vchan <- any.(payload.Payload)
	}
}

func (b blockSubscriber) OnSubscribe(ctx context.Context, subscription reactor.Subscription) {
	// workaround: watch context
	if ctx != context.Background() && ctx != context.TODO() {
		go func() {
			select {
			case <-ctx.Done():
				b.OnError(reactor.ErrSubscribeCancelled)
			case <-b.done:
			}
		}()
	}
	subscription.Request(reactor.RequestInfinite)
}
