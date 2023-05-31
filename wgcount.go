package main

import (
	"sync"
	"sync/atomic"
)

type waitGroupWithCount struct {
	sync.WaitGroup
	count atomic.Int64
}

func (wg *waitGroupWithCount) Add(delta int) {
	wg.count.Add(int64(delta))
	wg.WaitGroup.Add(delta)
}

func (wg *waitGroupWithCount) Done() {
	wg.WaitGroup.Done()
	wg.count.Add(-1)
}

func (wg *waitGroupWithCount) GetCount() int {
	return int(wg.count.Load())
}
