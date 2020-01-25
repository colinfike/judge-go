package judgego

import "sync"

type safeCache struct {
	sync.RWMutex
	m map[string]interface{}
}

func newSafeCache() *safeCache {
	return &safeCache{m: make(map[string]interface{})}
}
func (c *safeCache) get(name string) (interface{}, bool) {
	c.RLock()
	val, ok := c.m[name]
	c.RUnlock()
	return val, ok
}

func (c *safeCache) put(name string, data interface{}) {
	c.Lock()
	c.m[name] = data
	c.Unlock()
}
