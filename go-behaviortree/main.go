package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/joeycumines/go-behaviortree"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Info().Msg("Hello, World!")
	ExampleBackground_asyncJobQueue()
}

func debug(label string, tick Tick) Tick {
	return func(children []Node) (status Status, err error) {
		status, err = tick(children)
		log.Info().Msgf("%s returned (%v, %v)\n", label, status, err)
		return
	}
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
		ticker = NewTickerStopOnFailure(
			context.Background(),
			time.Millisecond,
			New(
				debug("sequence", Sequence),
				// no children
				New(debug("first node", func(children []Node) (Status, error) {
					select {
					case <-done:
						return Failure, nil
					default:
						return Success, nil
					}
				})),
				func() Node {
					// the tick is initialised once, and is stateful (though the tick it's wrapping isn't)
					tick := debug("bg", Background(func() Tick { return Selector }))
					return func() (Tick, []Node) {
						// this block will be refreshed each time that a new job is started
						var (
							job Job
						)
						return tick, []Node{
							New(
								debug("Seq", Sequence),
								Sync([]Node{
									New(func(children []Node) (Status, error) {
										select {
										case job = <-queue:
											running(1)
											return Success, nil
										default:
											return Failure, nil
										}
									}),
									New(Async(func(children []Node) (Status, error) {
										defer running(-1)
										doWorker(job)
										return Success, nil
									})),
								})...,
							),
							// no job available - success
							New(func(children []Node) (Status, error) {
								return Success, nil
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
func ExampleContext() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		btCtx = new(Context).WithTimeout(ctx, time.Millisecond*100)
		debug = func(args ...interface{}) Tick {
			return func([]Node) (Status, error) {
				fmt.Println(args...)
				return Success, nil
			}
		}
		counter      int
		counterEqual = func(v int) Tick {
			return func([]Node) (Status, error) {
				if counter == v {
					return Success, nil
				}
				return Failure, nil
			}
		}
		counterInc Tick = func([]Node) (Status, error) {
			counter++
			//fmt.Printf("counter = %d\n", counter)
			return Success, nil
		}
		ticker = NewTicker(ctx, time.Millisecond, New(
			Sequence,
			New(
				Selector,
				New(Not(btCtx.Err)),
				New(
					Sequence,
					New(debug(`(re)initialising btCtx...`)),
					New(btCtx.Init),
					New(Not(btCtx.Err)),
				),
			),
			New(
				Selector,
				New(
					Sequence,
					New(counterEqual(0)),
					New(debug(`blocking on context-enabled tick...`)),
					New(
						btCtx.Tick(func(ctx context.Context, children []Node) (Status, error) {
							fmt.Printf("NOTE children (%d) passed through\n", len(children))
							<-ctx.Done()
							return Success, nil
						}),
						New(Sequence),
						New(Sequence),
					),
					New(counterInc),
				),
				New(
					Sequence,
					New(counterEqual(1)),
					New(debug(`blocking on done...`)),
					New(btCtx.Done),
					New(counterInc),
				),
				New(
					Sequence,
					New(counterEqual(2)),
					New(debug(`canceling local then rechecking the above...`)),
					New(btCtx.Cancel),
					New(btCtx.Err),
					New(btCtx.Tick(func(ctx context.Context, children []Node) (Status, error) {
						<-ctx.Done()
						return Success, nil
					})),
					New(btCtx.Done),
					New(counterInc),
				),
				New(
					Sequence,
					New(counterEqual(3)),
					New(debug(`canceling parent then rechecking the above...`)),
					New(func([]Node) (Status, error) {
						cancel()
						return Success, nil
					}),
					New(btCtx.Err),
					New(btCtx.Tick(func(ctx context.Context, children []Node) (Status, error) {
						<-ctx.Done()
						return Success, nil
					})),
					New(btCtx.Done),
					New(debug(`exiting...`)),
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
