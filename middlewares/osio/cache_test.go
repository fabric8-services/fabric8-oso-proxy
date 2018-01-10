package middlewares

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func singleValResolver(val interface{}) Resolver {
	return func() (interface{}, error) {
		return val, nil
	}
}

func slowValResolver(duration time.Duration, val interface{}) Resolver {
	return func() (interface{}, error) {
		time.Sleep(duration)
		return val, nil
	}
}

func TestCacheReturnCachedDataSingleThread(t *testing.T) {
	c := Cache{}

	key := "A"
	value := "wee"

	first, _ := c.Get(key, singleValResolver(value)).Get()
	second, _ := c.Get(key, singleValResolver("wee2")).Get()

	assert.Equal(t, first, second)
}

func TestCacheReturnCachedDataMultiThread(t *testing.T) {
	c := Cache{}

	wg := sync.WaitGroup{}

	fetch := 10
	wg.Add(fetch + fetch)

	for i := 0; i < fetch; i++ {
		go func(index int) {
			val, err := c.Get("key1", slowValResolver(1*time.Second, "wee")).Get()
			fmt.Println("key1", val, err, index)
			wg.Done()
		}(i)

	}
	for i := 0; i < fetch; i++ {
		go func(index int) {
			key := fmt.Sprintf("key%v", index)
			v := fmt.Sprintf("wee%v", index)
			val, err := c.Get(key, singleValResolver(v)).Get()
			fmt.Println(key, val, err)
			wg.Done()
		}(i + 2)
	}

	wg.Wait()
}
