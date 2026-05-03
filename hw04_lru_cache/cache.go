package hw04lrucache

type Key string

type Cache interface {
	Set(key Key, value interface{}) bool
	Get(key Key) (interface{}, bool)
	Clear()
}

type lruCache struct {
	capacity int
	queue    List
	items    map[Key]*ListItem
}

type cacheItem struct {
	Key   Key
	Value interface{}
}

func (c *lruCache) Set(key Key, value interface{}) bool {

	if item, ok := c.items[key]; ok {
		item.Value = cacheItem{
			Key:   key,
			Value: value,
		}
		c.queue.MoveToFront(item)
		return true
	}
	item := c.queue.PushFront(cacheItem{
		Key:   key,
		Value: value,
	})
	c.items[key] = item
	if c.queue.Len() > c.capacity {
		oldest := c.queue.Back()
		c.queue.Remove(oldest)
		oldestItem := oldest.Value.(cacheItem)
		delete(c.items, oldestItem.Key)
	}
	return false
}

func (c *lruCache) Get(key Key) (interface{}, bool) {
	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.queue.MoveToFront(item)
	insideValue := item.Value.(cacheItem)
	return insideValue.Value, true
}

func (c *lruCache) Clear() {
	c.items = make(map[Key]*ListItem, c.capacity)
	c.queue = NewList()
}

func NewCache(capacity int) Cache {
	return &lruCache{
		capacity: capacity,
		queue:    NewList(),
		items:    make(map[Key]*ListItem, capacity),
	}
}
