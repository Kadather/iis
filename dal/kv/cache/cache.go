package cache

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coyove/common/lru"
	"github.com/gomodule/redigo/redis"
)

type batchGetTask struct {
	key     string
	r_value []byte
	r_ok    bool
	done    chan struct{}
}

type zset struct {
	v     string
	score int
}

type GlobalCache struct {
	local *lru.Cache
	Pool  *redis.Pool
	batch chan *batchGetTask
	zmu   sync.Mutex
	z     []*zset
}

type RedisConfig struct {
	Addr         string        `yaml:"Addr"`
	Timeout      time.Duration `yaml:"Timeout"`
	MaxIdle      int           `yaml:"MaxIdle"`
	BatchWorkers int
}

func NewGlobalCache(localSize int64, config *RedisConfig) *GlobalCache {
	gc := &GlobalCache{}
	gc.local = lru.NewCache(localSize)

	if config != nil && config.Addr != "" && os.Getenv("RC") != "0" {
		options := []redis.DialOption{}

		if config.Timeout == 0 {
			config.Timeout = time.Millisecond * 100
		}

		options = append(options, redis.DialConnectTimeout(config.Timeout))
		options = append(options, redis.DialReadTimeout(config.Timeout))
		options = append(options, redis.DialWriteTimeout(config.Timeout))

		if config.MaxIdle == 0 {
			config.MaxIdle = 10
		}

		if config.BatchWorkers == 0 {
			config.BatchWorkers = 1
		}

		gc.Pool = redis.NewPool(func() (redis.Conn, error) {
			return redis.Dial("tcp", config.Addr, options...)
		}, config.MaxIdle)

		gc.batch = make(chan *batchGetTask, localSize)

		for i := 0; i < config.BatchWorkers; i++ {
			go func() {
				tasks := []*batchGetTask{}
				blocking := false

				for {

					if blocking {
						t := <-gc.batch
						tasks = append(tasks, t)
					} else {
						for exit := false; !exit; {
							select {
							case t := <-gc.batch:
								tasks = append(tasks, t)
								if len(tasks) >= 16 {
									exit = true
								}
							default:
								exit = true
							}
						}
					}

					if len(tasks) == 0 {
						blocking = true
						continue
					}

					blocking = false

					keys := make([]interface{}, len(tasks))
					for i := range tasks {
						keys[i] = tasks[i].key
					}

					c := gc.Pool.Get()
					res, err := redis.Strings(c.Do("MGET", keys...))
					c.Close()

					if err != nil {
						log.Println("[GlobalCache_redis] batch get:", keys, "error:", err)
						for _, t := range tasks {
							t.r_ok = false
							t.done <- struct{}{}
						}
					} else {
						for i, t := range tasks {
							t.r_value = []byte(res[i])

							if bytes.HasSuffix(t.r_value, []byte("$")) {
								t.r_value = t.r_value[:len(t.r_value)-1]
								t.r_ok = true
							} else {
								t.r_value = nil
								t.r_ok = false
							}

							t.done <- struct{}{}
						}
					}

					tasks = tasks[:0]
				}
			}()
		}
	}

	return gc
}

func (gc *GlobalCache) Get(k string) ([]byte, bool) {
	// defer func(a time.Time) {
	// 	log.Println(time.Since(a))
	// }(time.Now())
	if gc.Pool == nil {
		v, _ := gc.local.Get(k)
		p, ok := v.([]byte)
		return p, ok
	}

	task := &batchGetTask{
		key:  k,
		done: make(chan struct{}, 1),
	}

	gc.batch <- task
	<-task.done
	return task.r_value, task.r_ok
}

func (gc *GlobalCache) Add(k string, v []byte) error {
	if gc.Pool == nil {
		gc.local.Add(k, v)
		return nil
	}

	c := gc.Pool.Get()
	defer c.Close()

	if _, err := c.Do("SET", k, append(v, '$')); err != nil {
		log.Println("[GlobalCache_redis] set:", k, "value:", string(v), "error:", err)
		return fmt.Errorf("cache error")
	}
	return nil
}

func (gc *GlobalCache) MGet(keys ...string) map[string][]byte {
	m := map[string][]byte{}
	if gc.Pool == nil {
		for _, k := range keys {
			v, _ := gc.local.Get(k)
			buf, ok := v.([]byte)
			if ok {
				m[k] = buf
			}
		}
		return m
	}

	c := gc.Pool.Get()
	defer c.Close()

	args := []interface{}{}
	for _, k := range keys {
		args = append(args, k)
	}

	res, err := redis.Strings(c.Do("MGET", args...))
	if err != nil {
		log.Println("[GlobalCache_redis] mget:", keys, "error:", err)
		return m
	}

	for i, r := range res {
		if strings.HasSuffix(r, "$") {
			m[keys[i]] = []byte(r[:len(r)-1])
		}
	}
	return m
}
