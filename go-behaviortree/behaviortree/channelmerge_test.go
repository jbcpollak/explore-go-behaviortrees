package behaviortree

import (
	"testing"
	"time"
)

func TestChannelMerge(t *testing.T) {

	var cm = NewChannelMerger()

	var c1 = make(chan int)

	cm.Add(c1)

	append := func() {
		for i := range 10 {
			c1 <- i
			time.Sleep(300 * time.Millisecond)
		}
		close(c1)
	}

	read := func() {
		for i := range cm.out {
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
}
