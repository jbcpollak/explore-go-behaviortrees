package behaviortree

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	bt "github.com/joeycumines/go-behaviortree"
)

type (
	Event struct {
		createdTime time.Time
	}

	// asyncTicker is a ticker that ticks in a separate goroutine
	asyncTicker struct {
		ctx         context.Context
		node        bt.Node
		tick_merger *ChannelMerger[Event]
		done        chan struct{}
		cancel      context.CancelFunc
	}
)

func NewAsyncTicker(ctx context.Context, root_node bt.Node) bt.Ticker {

	// I wonder if setting up the cancel should be left to the caller
	ctx, cancel := context.WithCancel(ctx)

	// setup the behavior for the event channels
	tick_events := make(chan Event)
	var tick_merger = NewChannelMerger(ctx, tick_events)
	ticker := &asyncTicker{
		ctx:         ctx,
		node:        root_node,
		tick_merger: tick_merger,
		done:        make(chan struct{}),
		cancel:      cancel,
	}

	run := func() {
		for evt := range tick_events {
			log.Debug().Msgf("ticking in event %v", evt)

			ticker.node.Tick()
		}
	}

	// Goroutine to wait till the ticker is done
	go func() {
		ticker.tick_merger.Wait()
		close(ticker.done)
	}()

	log.Debug().Msgf("Starting to Tick Behavior Tree")
	go run()

	return ticker
}

func (t *asyncTicker) Done() <-chan struct{} {
	return t.done
}

// Err implements behaviortree.Ticker.
func (t *asyncTicker) Err() error {
	panic("unimplemented")
}

// Stop implements behaviortree.Ticker.
func (t *asyncTicker) Stop() {
	t.cancel()
}
