package behaviortree

import (
	"context"
	"sync"
)

type ChannelMerger[T any] struct {
	ctx context.Context
	out chan<- T
	wg  sync.WaitGroup
}

func NewChannelMerger[T any](ctx context.Context, out chan<- T) *ChannelMerger[T] {
	return &ChannelMerger[T]{
		ctx,
		out,
		sync.WaitGroup{},
	}
}

func (cm *ChannelMerger[T]) Add(c <-chan T) {
	select {
	case <-cm.ctx.Done():
		return
	default:
	}
	cm.wg.Add(1)

	// Start a new output goroutine for the new input channel.
	output := func() {
		defer cm.wg.Done()
		for {
			var n T
			var ok bool
			select {
			case n, ok = <-c:
				if !ok {
					return
				}
			case <-cm.ctx.Done():
				return
			}

			select {
			case cm.out <- n:
			case <-cm.ctx.Done():
				return
			}
		}
	}

	go output()
}

// Cannot call this until all channels have been added
func (cm *ChannelMerger[T]) Wait() {
	cm.wg.Wait()
}
