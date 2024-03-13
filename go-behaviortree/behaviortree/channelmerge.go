package behaviortree

import "sync"

type ChannelMerger struct {
	out chan int
	cs  []chan<- int
	wg  sync.WaitGroup
}

func NewChannelMerger() *ChannelMerger {
	return &ChannelMerger{out: make(chan int)}
}

func (cm *ChannelMerger) Add(c chan int) {
	cm.wg.Add(1)
	cm.cs = append(cm.cs, c)

	// Start a new output goroutine for the new input channel in cs.  output
	output := func(c <-chan int) {
		for n := range c {
			cm.out <- n
		}
		cm.wg.Done()
	}

	go output(c)
}

// Cannot call this until all channels have been added
func (cm *ChannelMerger) Wait() {
	cm.wg.Wait()
	close(cm.out)
}
