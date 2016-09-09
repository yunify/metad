package util

import (
	"github.com/yunify/metad/atomic"
	"sync"
	"time"
)

type TimerPool struct {
	timeout  time.Duration
	pool     sync.Pool
	TotalNew atomic.AtomicInteger
	TotalGet atomic.AtomicInteger
}

func NewTimerPool(timeout time.Duration) *TimerPool {
	pool := sync.Pool{}
	totalNew := atomic.AtomicInteger(int32(0))
	pool.New = func() interface{} {
		t := time.NewTimer(timeout)
		totalNew.IncrementAndGet()
		//println("acquire timer from new", t, newCount)
		return t
	}
	return &TimerPool{timeout: timeout, pool: pool, TotalNew: totalNew, TotalGet: atomic.AtomicInteger(int32(0))}
}

func (tp *TimerPool) AcquireTimer() *time.Timer {
	tv := tp.pool.Get()
	t := tv.(*time.Timer)
	t.Reset(tp.timeout)
	tp.TotalGet.IncrementAndGet()
	//println("acquire timer", t, time.Now().Nanosecond()/1000/1000, getCount)
	return t
}

func (tp *TimerPool) ReleaseTimer(t *time.Timer) {
	if !t.Stop() {
		// Collect possibly added time from the channel
		// if timer has been stopped and nobody collected its' value.
		select {
		case <-t.C:
		default:
		}
	}
	//println("release timer", t, time.Now().Nanosecond()/1000/1000)
	tp.pool.Put(t)
}
