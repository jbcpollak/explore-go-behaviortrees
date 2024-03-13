package gobttest

import (
	"context"
	"fmt"
	"sync"
	"time"

	bt "github.com/joeycumines/go-behaviortree"
	"github.com/rs/zerolog/log"

	cbt "gobttest/behaviortree"
)

func main() {
	log.Info().Msg("Hello, World!")
	ExampleEventContext()
}

// ExampleBackground_asyncJobQueue implements a basic example of backgrounding of long-running tasks that may be
// performed concurrently, see ExampleNewTickerStopOnFailure_counter for an explanation of the ticker
func ExampleBackground_asyncJobQueue() {
	type (
		Job struct {
			Name     string
			Duration time.Duration
			Done     chan struct{}
		}
	)
	var (
		// doWorker performs the actual "work" for a Job
		doWorker = func(job Job) {
			fmt.Printf("[worker] job \"%s\" STARTED\n", job.Name)
			time.Sleep(job.Duration)
			fmt.Printf("[worker] job \"%s\" FINISHED\n", job.Name)
			close(job.Done)
		}
		// queue be sent jobs, which will be received within the ticker
		queue = make(chan Job, 50)
		// doClient sends and waits for a job
		doClient = func(name string, duration time.Duration) {
			job := Job{name, duration, make(chan struct{})}
			ts := time.Now()
			fmt.Printf("[client] job \"%s\" STARTED\n", job.Name)
			queue <- job
			<-job.Done
			fmt.Printf("[client] job \"%s\" FINISHED\n", job.Name)
			t := time.Since(ts)
			d := t - job.Duration
			if d < 0 {
				d *= -1
			}
			if d > time.Millisecond*50 {
				panic(fmt.Errorf(`job "%s" expected %s actual %s`, job.Name, job.Duration.String(), t.String()))
			}
		}
		// running keeps track of the number of running jobs
		running = func() func(delta int64) int64 {
			var (
				value int64
				mutex sync.Mutex
			)
			return func(delta int64) int64 {
				mutex.Lock()
				defer mutex.Unlock()
				value += delta
				return value
			}
		}()
		// done will be closed when it's time to exit the ticker
		done   = make(chan struct{})
		ticker = bt.NewTickerStopOnFailure(
			context.Background(),
			time.Millisecond,
			bt.New(
				cbt.Debug("sequence", bt.Sequence),
				// no children
				bt.New(cbt.Debug("first node", func(children []bt.Node) (bt.Status, error) {
					select {
					case <-done:
						return bt.Failure, nil
					default:
						return bt.Success, nil
					}
				})),
				func() bt.Node {
					// the tick is initialised once, and is stateful (though the tick it's wrapping isn't)
					tick := cbt.Debug("bg", bt.Background(func() bt.Tick { return bt.Selector }))
					return func() (bt.Tick, []bt.Node) {
						// this block will be refreshed each time that a new job is started
						var (
							job Job
						)
						return tick, []bt.Node{
							bt.New(
								cbt.Debug("Seq", bt.Sequence),
								bt.Sync([]bt.Node{
									bt.New(func(children []bt.Node) (bt.Status, error) {
										select {
										case job = <-queue:
											running(1)
											return bt.Success, nil
										default:
											return bt.Failure, nil
										}
									}),
									bt.New(bt.Async(func(children []bt.Node) (bt.Status, error) {
										defer running(-1)
										doWorker(job)
										return bt.Success, nil
									})),
								})...,
							),
							// no job available - success
							bt.New(func(children []bt.Node) (bt.Status, error) {
								return bt.Success, nil
							}),
						}
					}
				}(),
			),
		)
		wg sync.WaitGroup
	)
	wg.Add(1)
	run := func(name string, duration time.Duration) {
		wg.Add(1)
		defer wg.Done()
		doClient(name, duration)
	}

	fmt.Printf("running jobs: %d\n", running(0))

	go run(`1. 120ms`, time.Millisecond*120)
	time.Sleep(time.Millisecond * 25)
	go run(`2. 70ms`, time.Millisecond*70)
	time.Sleep(time.Millisecond * 25)
	fmt.Printf("running jobs: %d\n", running(0))

	doClient(`3. 150ms`, time.Millisecond*150)
	time.Sleep(time.Millisecond * 50)
	fmt.Printf("running jobs: %d\n", running(0))

	time.Sleep(time.Millisecond * 50)
	wg.Done()
	wg.Wait()
	close(done)
	<-ticker.Done()
	if err := ticker.Err(); err != nil {
		panic(err)
	}
	//output:
	//running jobs: 0
	//[client] job "1. 120ms" STARTED
	//[worker] job "1. 120ms" STARTED
	//[client] job "2. 70ms" STARTED
	//[worker] job "2. 70ms" STARTED
	//running jobs: 2
	//[client] job "3. 150ms" STARTED
	//[worker] job "3. 150ms" STARTED
	//[worker] job "2. 70ms" FINISHED
	//[client] job "2. 70ms" FINISHED
	//[worker] job "1. 120ms" FINISHED
	//[client] job "1. 120ms" FINISHED
	//[worker] job "3. 150ms" FINISHED
	//[client] job "3. 150ms" FINISHED
	//running jobs: 0
}

// ExampleContext demonstrates how the Context implementation may be used to integrate with the context package
func ExampleEventContext() {
	tick_events := make(chan cbt.Event)
	tick_merger := cbt.NewChannelMerger(tick_events)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		btCtx = new(bt.Context).WithTimeout(ctx, time.Millisecond*100)
		debug = func(args ...interface{}) bt.Tick {
			return func([]bt.Node) (bt.Status, error) {
				fmt.Println(args...)
				return bt.Success, nil
			}
		}
		counter      int
		counterEqual = func(v int) bt.Tick {
			return func([]bt.Node) (bt.Status, error) {
				if counter == v {
					return bt.Success, nil
				}
				return bt.Failure, nil
			}
		}
		counterInc bt.Tick = func([]bt.Node) (bt.Status, error) {
			counter++
			//fmt.Printf("counter = %d\n", counter)
			return bt.Success, nil
		}
		ticker = cbt.NewAsyncTicker(ctx, bt.New(
			bt.Sequence,
			bt.New(
				bt.Selector,
				bt.New(bt.Not(btCtx.Err)),
				bt.New(
					bt.Sequence,
					bt.New(debug(`(re)initialising btCtx...`)),
					bt.New(btCtx.Init),
					bt.New(bt.Not(btCtx.Err)),
				),
			),
			bt.New(
				bt.Selector,
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(0)),
					bt.New(debug(`blocking on context-enabled tick...`)),
					bt.New(
						btCtx.Tick(func(ctx context.Context, children []bt.Node) (bt.Status, error) {
							fmt.Printf("NOTE children (%d) passed through\n", len(children))
							<-ctx.Done()
							return bt.Success, nil
						}),
						bt.New(bt.Sequence),
						bt.New(bt.Sequence),
					),
					bt.New(counterInc),
				),
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(1)),
					bt.New(debug(`blocking on done...`)),
					bt.New(btCtx.Done),
					bt.New(counterInc),
				),
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(2)),
					bt.New(debug(`canceling local then rechecking the above...`)),
					bt.New(btCtx.Cancel),
					bt.New(btCtx.Err),
					bt.New(btCtx.Tick(func(ctx context.Context, children []bt.Node) (bt.Status, error) {
						<-ctx.Done()
						return bt.Success, nil
					})),
					bt.New(btCtx.Done),
					bt.New(counterInc),
				),
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(3)),
					bt.New(debug(`canceling parent then rechecking the above...`)),
					bt.New(func([]bt.Node) (bt.Status, error) {
						cancel()
						return bt.Success, nil
					}),
					bt.New(btCtx.Err),
					bt.New(btCtx.Tick(func(ctx context.Context, children []bt.Node) (bt.Status, error) {
						<-ctx.Done()
						return bt.Success, nil
					})),
					bt.New(btCtx.Done),
					bt.New(debug(`exiting...`)),
				),
			),
		))
	)

	<-ticker.Done()

	//output:
	//(re)initialising btCtx...
	//blocking on context-enabled tick...
	//NOTE children (2) passed through
	//(re)initialising btCtx...
	//blocking on done...
	//(re)initialising btCtx...
	//canceling local then rechecking the above...
	//(re)initialising btCtx...
	//canceling parent then rechecking the above...
	//exiting...
}

// ExampleContext demonstrates how the Context implementation may be used to integrate with the context package
func ExampleContext() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		btCtx = new(bt.Context).WithTimeout(ctx, time.Millisecond*100)
		debug = func(args ...interface{}) bt.Tick {
			return func([]bt.Node) (bt.Status, error) {
				fmt.Println(args...)
				return bt.Success, nil
			}
		}
		counter      int
		counterEqual = func(v int) bt.Tick {
			return func([]bt.Node) (bt.Status, error) {
				if counter == v {
					return bt.Success, nil
				}
				return bt.Failure, nil
			}
		}
		counterInc bt.Tick = func([]bt.Node) (bt.Status, error) {
			counter++
			//fmt.Printf("counter = %d\n", counter)
			return bt.Success, nil
		}
		ticker = bt.NewTicker(ctx, time.Millisecond, bt.New(
			bt.Sequence,
			bt.New(
				bt.Selector,
				bt.New(bt.Not(btCtx.Err)),
				bt.New(
					bt.Sequence,
					bt.New(debug(`(re)initialising btCtx...`)),
					bt.New(btCtx.Init),
					bt.New(bt.Not(btCtx.Err)),
				),
			),
			bt.New(
				bt.Selector,
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(0)),
					bt.New(debug(`blocking on context-enabled tick...`)),
					bt.New(
						btCtx.Tick(func(ctx context.Context, children []bt.Node) (bt.Status, error) {
							fmt.Printf("NOTE children (%d) passed through\n", len(children))
							<-ctx.Done()
							return bt.Success, nil
						}),
						bt.New(bt.Sequence),
						bt.New(bt.Sequence),
					),
					bt.New(counterInc),
				),
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(1)),
					bt.New(debug(`blocking on done...`)),
					bt.New(btCtx.Done),
					bt.New(counterInc),
				),
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(2)),
					bt.New(debug(`canceling local then rechecking the above...`)),
					bt.New(btCtx.Cancel),
					bt.New(btCtx.Err),
					bt.New(btCtx.Tick(func(ctx context.Context, children []bt.Node) (bt.Status, error) {
						<-ctx.Done()
						return bt.Success, nil
					})),
					bt.New(btCtx.Done),
					bt.New(counterInc),
				),
				bt.New(
					bt.Sequence,
					bt.New(counterEqual(3)),
					bt.New(debug(`canceling parent then rechecking the above...`)),
					bt.New(func([]bt.Node) (bt.Status, error) {
						cancel()
						return bt.Success, nil
					}),
					bt.New(btCtx.Err),
					bt.New(btCtx.Tick(func(ctx context.Context, children []bt.Node) (bt.Status, error) {
						<-ctx.Done()
						return bt.Success, nil
					})),
					bt.New(btCtx.Done),
					bt.New(debug(`exiting...`)),
				),
			),
		))
	)

	<-ticker.Done()

	//output:
	//(re)initialising btCtx...
	//blocking on context-enabled tick...
	//NOTE children (2) passed through
	//(re)initialising btCtx...
	//blocking on done...
	//(re)initialising btCtx...
	//canceling local then rechecking the above...
	//(re)initialising btCtx...
	//canceling parent then rechecking the above...
	//exiting...
}
