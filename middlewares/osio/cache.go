package middlewares

import (
	"sync"
)

type Cache struct {
	mux sync.Mutex
	m   map[string]Promise
}

func (c *Cache) Get(key string, resolver Resolver) Promise {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.m == nil {
		c.m = make(map[string]Promise)
	}

	if val, ok := c.m[key]; ok {
		return val
	}
	val := &ResolverPromise{resolver: resolver}
	c.m[key] = val
	return val
}

type ResolverPromise struct {
	mux      sync.Mutex
	resolver Resolver
	resolved bool
	value    interface{}
	err      error
}

func (r *ResolverPromise) Get() (interface{}, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.resolved {
		return r.value, r.err
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func(wg *sync.WaitGroup, resolver Resolver) {
		r.value, r.err = resolver()
		wg.Done()

	}(&wg, r.resolver)
	wg.Wait()
	r.resolved = true

	return r.value, r.err
}

type Resolver func() (interface{}, error)

type Promise interface {
	Get() (interface{}, error)
}
