package hw04lrucache

type List interface {
	Len() int
	Front() *ListItem
	Back() *ListItem
	PushFront(v interface{}) *ListItem
	PushBack(v interface{}) *ListItem
	Remove(i *ListItem)
	MoveToFront(i *ListItem)
}

type ListItem struct {
	Value interface{}
	Next  *ListItem
	Prev  *ListItem
}

type list struct {
	length int
	back   *ListItem
	front  *ListItem
}

func (l *list) Len() int {
	return l.length
}

func (l *list) Front() *ListItem {
	return l.front
}

func (l *list) tryInitWithItem(item *ListItem) bool {
	if l.length != 0 {
		return false
	}
	l.front = item
	l.back = item
	l.length = 1
	return true
}

func (l *list) PushFront(v interface{}) *ListItem {
	item := &ListItem{Value: v, Next: l.front}
	if l.tryInitWithItem(item) {
		return item
	}
	l.front.Prev = item
	l.front = item
	l.length++
	return item
}

func (l *list) PushBack(v interface{}) *ListItem {
	item := &ListItem{Value: v, Prev: l.back}
	if l.tryInitWithItem(item) {
		return item
	}
	l.back.Next = item
	l.back = item
	l.length++
	return item
}

func (l *list) Back() *ListItem {
	return l.back
}

func (l *list) Remove(item *ListItem) {
	if item == nil {
		return
	}
	if item.Prev != nil {
		item.Prev.Next = item.Next
	} else {
		l.front = item.Next
	}
	if item.Next != nil {
		item.Next.Prev = item.Prev
	} else {
		l.back = item.Prev
	}
	item.Next = nil
	item.Prev = nil
	l.length--
}

func (l *list) MoveToFront(item *ListItem) {
	if item == nil || item == l.front {
		return
	}
	l.Remove(item)
	item.Next = l.front
	item.Prev = nil
	if l.front != nil {
		l.front.Prev = item
	}
	l.front = item
	if l.back == nil {
		l.back = item
	}
	l.length++
}

func NewList() List {
	return new(list)
}
