// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package atomic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAtomicIncrement(t *testing.T) {
	i := AtomicInteger(int32(0))
	v := i.IncrementAndGet()
	assert.Equal(t, int32(1), v)
	assert.Equal(t, int32(1), i.Get())
	v = i.GetAndIncrement()
	assert.Equal(t, int32(1), v)
	assert.Equal(t, int32(2), i.Get())
}

func TestAtomicDecrement(t *testing.T) {
	i := AtomicInteger(int32(2))
	v := i.DecrementAndGet()
	assert.Equal(t, int32(1), v)
	assert.Equal(t, int32(1), i.Get())
	v = i.GetAndDecrement()
	assert.Equal(t, int32(1), v)
	assert.Equal(t, int32(0), i.Get())
}

func TestAtomicConcurrentIncrement(t *testing.T) {
	integer := AtomicInteger(int32(0))
	count := 100
	wait := sync.WaitGroup{}
	wait.Add(count)
	start := sync.WaitGroup{}
	start.Add(1)
	for i := 0; i < count; i++ {
		go func() {
			start.Wait()
			integer.IncrementAndGet()
			wait.Done()
		}()
	}
	start.Done()
	wait.Wait()
	assert.Equal(t, int32(count), integer.Get())
}

func TestAtomicConcurrentDecrement(t *testing.T) {
	count := 100
	integer := AtomicInteger(int32(count))
	wait := sync.WaitGroup{}
	wait.Add(count)
	start := sync.WaitGroup{}
	start.Add(1)
	for i := 0; i < count; i++ {
		go func() {
			start.Wait()
			integer.DecrementAndGet()
			wait.Done()
		}()
	}
	start.Done()
	wait.Wait()
	assert.Equal(t, int32(0), integer.Get())
}

func TestAtomicConcurrentIncrementAndDecrementAndGet(t *testing.T) {
	count := 100
	integer := AtomicInteger(0)
	wait := sync.WaitGroup{}
	wait.Add(count)
	start := sync.WaitGroup{}
	start.Add(1)
	for i := 0; i < count; i++ {
		go func(idx int) {
			start.Wait()
			if idx%2 == 0 {
				integer.IncrementAndGet()
			} else {
				integer.DecrementAndGet()
			}
			integer.Get()
			wait.Done()
		}(i)
	}
	start.Done()
	wait.Wait()
	assert.Equal(t, int32(0), integer.Get())
}

func TestAtomicConcurrentGetAndIncrementAndDecrement(t *testing.T) {
	count := 100
	integer := AtomicInteger(0)
	wait := sync.WaitGroup{}
	wait.Add(count)
	start := sync.WaitGroup{}
	start.Add(1)
	for i := 0; i < count; i++ {
		go func(idx int) {
			start.Wait()
			if idx%2 == 0 {
				integer.GetAndIncrement()
			} else {
				integer.GetAndDecrement()
			}
			integer.Get()
			wait.Done()
		}(i)
	}
	start.Done()
	wait.Wait()
	assert.Equal(t, int32(0), integer.Get())
}

func TestAtomicIncrementLong(t *testing.T) {
	i := AtomicLong(int64(0))
	v := i.IncrementAndGet()
	assert.Equal(t, int64(1), v)
	assert.Equal(t, int64(1), i.Get())
	v = i.GetAndIncrement()
	assert.Equal(t, int64(1), v)
	assert.Equal(t, int64(2), i.Get())
}

func TestAtomicConcurrentIncrementAndDecrementLong(t *testing.T) {
	count := 100
	long := AtomicLong(0)
	wait := sync.WaitGroup{}
	wait.Add(count)
	start := sync.WaitGroup{}
	start.Add(1)
	for i := 0; i < count; i++ {
		go func(idx int) {
			start.Wait()
			if idx%2 == 0 {
				long.IncrementAndGet()
			} else {
				long.DecrementAndGet()
			}
			long.Get()
			wait.Done()
		}(i)
	}
	start.Done()
	wait.Wait()
	assert.Equal(t, int64(0), long.Get())
}
