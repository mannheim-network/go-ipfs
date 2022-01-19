package utils

import (
	"sync"
)

type Lpool struct {
	queue chan int
	wg    *sync.WaitGroup
}

func NewLpool(size int) *Lpool {
	if size <= 0 {
		size = 1
	}
	return &Lpool{
		queue: make(chan int, size),
		wg:    &sync.WaitGroup{},
	}
}

func (p *Lpool) Add(delta int) {
	for i := 0; i < delta; i++ {
		p.queue <- 1
	}
	for i := 0; i > delta; i-- {
		<-p.queue
	}
	p.wg.Add(delta)
}

func (p *Lpool) Done() {
	<-p.queue
	p.wg.Done()
}

func (p *Lpool) Wait() {
	p.wg.Wait()
}
