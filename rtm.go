// # Resident Task Manager
//
// Provides in-process task execution, using go-routines.
package rtm

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/eapache/queue"
)

const (
	DEFAULT_MAX_TASKS = 100
)

type TaskExecutor struct {
	poolSize    int
	queue       chan *TaskContext
	poolCounter *atomic.Int32
	backbuffer  *queue.Queue
	mutex       *sync.Mutex
	rootCtx     context.Context
	cancelFn    context.CancelFunc
	shutdown    *atomic.Bool
	wg          sync.WaitGroup
}

// Create a new, one time use executor with a working pool of size "size".
//
// Queueing tasks to the pool will cause the task to immediately run if the
// pool size is within the specified size, otherwise tasks are scheduled on
// a FIFO list, for later execution when space becomes available in the pool.
func NewTaskExecutor(size ...int) *TaskExecutor {
	ctx, fn := context.WithCancel(context.Background())
	rpm := &TaskExecutor{
		poolCounter: &atomic.Int32{},
		backbuffer:  queue.New(),
		mutex:       &sync.Mutex{},
		rootCtx:     ctx,
		cancelFn:    fn,
		shutdown:    &atomic.Bool{},
	}
	if len(size) == 0 {
		rpm.poolSize = DEFAULT_MAX_TASKS
	} else {
		rpm.poolSize = size[0]
		if rpm.poolSize <= 0 {
			rpm.poolSize = DEFAULT_MAX_TASKS
		}
	}
	rpm.queue = make(chan *TaskContext)
	go func() {
		for {
			select {
			case <-rpm.rootCtx.Done():
				//_wait := rpm.poolCounter.Load()
				// _buffered := rpm.backbuffer.Length()
				//fmt.Printf("Cancelled. Wait for %v routines to finish. %v in queue\n", _wait, _buffered)
				return
			case ctx := <-rpm.queue:
				{
					if rpm.shutdown.Load() { // shutdown or in process of shutting down
						break
					}

					if rpm.full() {
						// execution pool is full. push to backup queue
						rpm.backbuffer.Add(ctx)
					} else {
						rpm.poolCounter.Add(1)
						rpm.wg.Add(1)
						go func() {
							defer func() {
								_ = recover()
								rpm.poolCounter.Add(-1)
								rpm.wg.Done()
							}()
							if !rpm.shutdown.Load() {
								ctx.fn(ctx)
								// pop from backup queue to executor
								if !rpm.shutdown.Load() {
									rpm.enqueueFromBackBuffer()
								}
							}
						}()
					}
				}
			}
		}
	}()
	return rpm
}

func (te *TaskExecutor) enqueueFromBackBuffer() {
	te.mutex.Lock()
	defer te.mutex.Unlock()
	if te.backbuffer.Length() > 0 {
		proc := te.backbuffer.Remove()
		te.queue <- proc.(*TaskContext)
	}
}

// checks if working pool is full
func (te *TaskExecutor) full() bool {
	return int(te.poolCounter.Load()) >= te.poolSize
}

// Shuts down the executor and any running tasks.
//
// The function will block until all running tasks in the pool have finished.
//
// Tasks in the backup queue do not affect this function and are ignored.
func (te *TaskExecutor) Shutdown() {
	if te.shutdown.CompareAndSwap(false, true) {
		close(te.queue)
		te.cancelFn()
		te.wg.Wait()
	}
}

// Queue the task to run immediately or later when execution space becomes available.
func (te *TaskExecutor) Queue(fn TaskFunc, args TaskArguments) {
	te.queue <- &TaskContext{fn: fn, Args: args, Ctx: te.rootCtx}
}

type TaskContext struct {
	// Cancellable context. One can also check the status of the context (Ctx.Err()) to check if the context
	// has been cancelled
	Ctx context.Context
	// Arguments passed to the task
	Args TaskArguments
	fn   TaskFunc
}

type TaskArguments map[string]any

// The task function to be executed by the task manager
type TaskFunc func(ctx *TaskContext)
