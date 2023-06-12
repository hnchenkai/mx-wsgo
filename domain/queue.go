package domain

import "sync"

type Queue struct {
	elements []interface{}
	lock     *sync.Mutex
}

func NewQueue() *Queue {
	return &Queue{
		elements: make([]interface{}, 0),
		lock:     &sync.Mutex{},
	}
}

func (q *Queue) Size() int {
	return len(q.elements)
}

func (q *Queue) Pop() (ele interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	size := q.Size()
	if size == 0 {
		return nil
	}
	ele = q.elements[size-1]
	q.elements = q.elements[:size-1]
	return
}

// IndexOf 查找元素的位置 并且返回总大小
func (q *Queue) IndexOf(ele interface{}) (int64, int64) {
	q.lock.Lock()
	defer q.lock.Unlock()
	size := q.Size()
	if size == 0 {
		return -1, 0
	}
	for i, v := range q.elements {
		if v == ele {
			return int64(i), int64(size)
		}
	}
	return -1, int64(size)
}

func (q *Queue) Shift() (ele interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	size := q.Size()
	if size == 0 {
		return nil
	}
	ele = q.elements[0]
	q.elements = q.elements[1:]
	return
}

func (q *Queue) Add(ele interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = append(q.elements, ele)
}

func (q *Queue) Del(ele interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()

	for i, v := range q.elements {
		if v == ele {
			q.elements = append(q.elements[:i], q.elements[i+1:]...)
			return
		}
	}
}
