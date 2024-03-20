# rtm

Resident Task Manager for golang

#### Use case

You want to run parallel background tasks and you don't care about in memory data loss.

#### Goodies

- ✅ Offers parallel task execution by default
- ✅ Refined cancellation mechanism to prevent runaway go-routines and leaks.
- ✅ Crash recovery guarantees misbehaving and naughty tasks get booted off the pool
- ✅ Unbounded FIFO ring-buffer based backup [queue](https://github.com/eapache/queue) so you can run tasks later when execution pool size drops.

#### Caveats

- ❌ In Memory Only: Tasks and data do not persist.
- ❌ When cancelling task execution, the calling thread will block indefinitely to wait for all tasks in the executiono pool to finish. Note that tasks on the backup queue are simply dicarded since they never executed.

**Example**

```go
package main

import (
	"fmt"
	"github.com/gintec-rdl/rtm"
	"math/rand"
	"os"
	"time"
)

func main() {
	tasks := 100
    poolSize := 5
	te := rtm.NewTaskExecutor(poolSize)

	task := func(ctx *rtm.TaskContext) {
		index := ctx.Args["index"].(int)
		fmt.Printf("[%d] enter\n", index)
        // perform some time consuming action
		time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)

        // simulate a crash
        if rand.Intn(2) == 1 {
			panic(fmt.Errorf("[%d] crashed", index))
		}

		fmt.Printf("[%d] leave\n", index)
	}

	for i := 0; i < tasks; i++ {
		params := rtm.TaskArguments{"index": i}
		te.Queue(task, params)
	}

	time.Sleep(5 * time.Second)
	te.Shutdown()
}
```
