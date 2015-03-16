package session

import "sync"
import "github.com/cuixin/goalg/queue"

type SafeQueue struct {
	sync.Mutex
	q *queue.Queue
}

func NewSafeQueue() *SafeQueue {
	return &SafeQueue{q: queue.New()}
}

func (self *SafeQueue) In(v interface{}) {
	self.Lock()
	self.q.Enqueue(v)
	self.Unlock()
}

func (self *SafeQueue) Out() interface{} {
	var v interface{}
	self.Lock()
	v = self.q.Dequeue()
	self.Unlock()
	return v
}

func (self *SafeQueue) Clean() []interface{} {
	var retQueue []interface{}
	self.Lock()
	qLen := self.q.Len()
	if qLen > 0 {
		retQueue = make([]interface{}, 0, qLen)
		for {
			front := self.q.Dequeue()
			if front == nil {
				break
			}
			retQueue = append(retQueue, front)
		}
	}
	self.Unlock()
	return retQueue
}
