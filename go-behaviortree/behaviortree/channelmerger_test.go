package behaviortree

import (
	"context"
	"testing"
	"time"
)

func TestChannelMerge(t *testing.T) {

	ctx, _ := context.WithCancel(context.Background())
	out := make(chan int)
	var cm = NewChannelMerger(out)

	// At init time, each node would do this
	var c1 = make(chan int)
	cm.Add(ctx, c1)

	// Goroutine internal to node that would be
	// executed at runtime
	append := func() {
		for i := range 10 {
			c1 <- i
			time.Sleep(100 * time.Millisecond)
		}

		// in some situations close might be called
		// separately as part of a shutdown phase
		// the current go-behaviortree library doesn't
		// seem to support that afaik.
		close(c1)
	}

	read := func() {
		for i := range out {
			// in production, tick on events here
			t.Log(i)
		}
	}

	t.Log("reading")
	go read()

	t.Log("appending")
	go append()

	t.Log("waiting")
	cm.Wait()

	t.Log("shutdown complete")
}
