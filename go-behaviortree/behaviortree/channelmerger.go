package behaviortree

import (
	"context"
	"sync"
)

type ChannelMerger[T any] struct {
	out chan<- T
	wg  sync.WaitGroup
}

func NewChannelMerger[T any](out chan<- T) *ChannelMerger[T] {
	return &ChannelMerger[T]{out: out}
}

func (cm *ChannelMerger[T]) Add(ctx context.Context, c <-chan T) {
	select {
	case <-ctx.Done():
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
			case <-ctx.Done():
				return
			}

			select {
			case cm.out <- n:
			case <-ctx.Done():
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
