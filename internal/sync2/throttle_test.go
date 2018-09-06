package sync2

import (
	"fmt"
	"io"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func ExampleThrottle() {
	throttle := NewThrottle()
	var wg sync.WaitGroup

	// consumer
	go func() {
		defer wg.Done()
		totalConsumed := int64(0)
		for {
			available, err := throttle.ConsumeOrWait(8)
			if err != nil {
				return
			}
			fmt.Println("- consuming ", available, " total=", totalConsumed)
			totalConsumed += available

			// do work for available amount
			time.Sleep(time.Duration(available) * time.Millisecond)
		}
	}()

	// producer
	go func() {
		defer wg.Done()

		step := int64(8)
		for total := int64(64); total >= 0; total -= step {
			err := throttle.ProduceAndWaitUntilBelow(step, step*3)
			if err != nil {
				return
			}

			fmt.Println("+ producing", step, " left=", total)
			time.Sleep(time.Duration(rand.Intn(8)) * time.Millisecond)
		}

		throttle.Fail(io.EOF)
	}()

	wg.Wait()

	fmt.Println("done", throttle.Err())
}

func TestThrottleBasic(t *testing.T) {
	throttle := NewThrottle()
	var stage int64

	// consumer
	go func() {
		consume, _ := throttle.ConsumeOrWait(4)
		if v := atomic.LoadInt64(&stage); v != 1 || consume != 4 {
			t.Fatalf("did not block in time: %d / %d", v, consume)
		}

		consume, _ = throttle.ConsumeOrWait(4)
		if v := atomic.LoadInt64(&stage); v != 1 || consume != 4 {
			t.Fatalf("did not block in time: %d / %d", v, consume)
		}
		atomic.AddInt64(&stage, 2)
	}()

	// slowly produce
	time.Sleep(time.Millisecond)
	// set stage to 1
	atomic.AddInt64(&stage, 1)
	_ = throttle.Produce(8)
	// wait until consumer consumes twice
	_ = throttle.WaitUntilBelow(3)
	// wait slightly for updating stage
	time.Sleep(time.Millisecond)

	if v := atomic.LoadInt64(&stage); v != 3 {
		t.Fatalf("did not unblock in time: %d", v)
	}
}