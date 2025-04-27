package esiproc

import "sync"

type queue[T any] struct {
	cond chan struct{}

	itemsMu sync.Mutex
	items   []T
}

func newQueue[T any]() *queue[T] {
	return &queue[T]{
		cond: make(chan struct{}, 1),
	}
}

func (q *queue[T]) pop(done <-chan struct{}) (T, bool) {
	for {
		select {
		case <-done:
			var zeroT T
			return zeroT, false
		case <-q.cond:
		}

		q.itemsMu.Lock()

		// Someone else was faster, try again
		if len(q.items) == 0 {
			q.itemsMu.Unlock()
			continue
		}

		item, more := q.items[0], len(q.items) > 1

		q.items = q.items[1:]
		q.itemsMu.Unlock()

		if more {
			q.signal()
		}

		return item, true
	}
}

func (q *queue[T]) push(item T) {
	q.itemsMu.Lock()
	q.items = append(q.items, item)
	q.itemsMu.Unlock()

	q.signal()
}

func (q *queue[T]) signal() {
	select {
	case q.cond <- struct{}{}:
	default:
	}
}
