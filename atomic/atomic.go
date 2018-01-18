// Copyright 2018 Yunify Inc. All rights reserved.
// Use of this source code is governed by a Apache license
// that can be found in the LICENSE file.

package atomic

import "sync/atomic"

// AtomicInteger is a int32 wrapper fo atomic
type AtomicInteger int32

// IncrementAndGet increment wrapped int32 with 1 and return new value.
func (i *AtomicInteger) IncrementAndGet() int32 {
	return atomic.AddInt32((*int32)(i), int32(1))
}

// GetAndIncrement increment wrapped int32 with 1 and return old value.
func (i *AtomicInteger) GetAndIncrement() int32 {
	ret := atomic.LoadInt32((*int32)(i))
	atomic.AddInt32((*int32)(i), int32(1))
	return ret
}

// DecrementAndGet decrement wrapped int32 with 1 and return new value.
func (i *AtomicInteger) DecrementAndGet() int32 {
	return atomic.AddInt32((*int32)(i), int32(-1))
}

// GetAndDecrement decrement wrapped int32 with 1 and return old value.
func (i *AtomicInteger) GetAndDecrement() int32 {
	ret := atomic.LoadInt32((*int32)(i))
	atomic.AddInt32((*int32)(i), int32(-1))
	return ret
}

// Get current value
func (i *AtomicInteger) Get() int32 {
	return atomic.LoadInt32((*int32)(i))
}

// AtomicInteger is a int32 wrapper fo atomic
type AtomicLong int64

// IncrementAndGet increment wrapped int64 with 1 and return new value.
func (i *AtomicLong) IncrementAndGet() int64 {
	return atomic.AddInt64((*int64)(i), int64(1))
}

// GetAndIncrement increment wrapped int64 with 1 and return old value.
func (i *AtomicLong) GetAndIncrement() int64 {
	ret := int64(*i)
	atomic.AddInt64((*int64)(i), int64(1))
	return ret
}

// DecrementAndGet decrement wrapped int64 with 1 and return new value.
func (i *AtomicLong) DecrementAndGet() int64 {
	return atomic.AddInt64((*int64)(i), int64(-1))
}

// GetAndDecrement decrement wrapped int64 with 1 and return old value.
func (i *AtomicLong) GetAndDecrement() int64 {
	ret := int64(*i)
	atomic.AddInt64((*int64)(i), int64(-1))
	return ret
}

// Get current value
func (i *AtomicLong) Get() int64 {
	return atomic.LoadInt64((*int64)(i))
}
