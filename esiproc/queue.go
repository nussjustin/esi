package esiproc

import "sync"

type queue[T any] struct {
	set chan struct{}

	mu sync.Mutex
	s  []T
}

func (q *queue[T]) init() {
	q.set = make(chan struct{}, 1)
}

func (q *queue[T]) pop(cancel <-chan struct{}) (T, bool) {
	for {
		select {
		case <-cancel:
			var zeroT T
			return zeroT, false
		case <-q.set:
		}

		q.mu.Lock()
		if len(q.s) == 0 {
			q.mu.Unlock()
			continue
		}

		t, more := q.s[0], len(q.s) > 1

		q.s = q.s[1:]
		q.mu.Unlock()

		if more {
			select {
			case q.set <- struct{}{}:
			default:
			}
		}

		return t, true
	}
}

func (q *queue[T]) push(t T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.s = append(q.s, t)

	select {
	case q.set <- struct{}{}:
	default:
	}
}
