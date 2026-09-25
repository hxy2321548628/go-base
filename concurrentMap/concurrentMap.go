package concurrentmap

import (
	"context"
	"sync"
	"time"
)

/*
实现一个map

1. 面向高并发
2. 只存在插入和查询操作 O(1)
3. 查询时, 如果 key 存在, 直接返回 value. 若不存在, 阻塞直到 key 存在时返回对应的 val. 等待指定时长仍然未放入, 返回超时错误
4. 不能有死锁或 panic 风险

*/

/*
通道关闭会通知所有监听状态的 goroutine
读取的话只会通知一个
*/

type myChan struct {
	ch chan struct{}
	sync.Once
}

func (mc *myChan) Close() {
	mc.Do(func() {
		close(mc.ch)
	})
}

func NewmyChan() *myChan {
	return &myChan{
		ch: make(chan struct{}),
	}
}

type ConcurrentMap struct {
	sync.Mutex
	conMap   map[int]int     // 存放真实值的字典
	key2Chan map[int]*myChan // 注意这里存的一定是指针类型
}

func NewConcurrentMap() *ConcurrentMap {
	return &ConcurrentMap{
		conMap:   make(map[int]int),
		key2Chan: make(map[int]*myChan),
	}
}

func (cm *ConcurrentMap) Set(key, value int) {
	cm.Lock()
	defer cm.Unlock()

	cm.conMap[key] = value

	ch := cm.key2Chan[key]

	if ch != nil {
		ch.Close()
	}
}

func (cm *ConcurrentMap) Get(key int, duration time.Duration) (int, error) {

	// 先上锁查询
	cm.Lock()

	if val, ok := cm.conMap[key]; ok {
		cm.Unlock()
		return val, nil
	}

	// 不存在则等待
	// 先查看是否已经有在等待的
	ch, ok := cm.key2Chan[key]
	if !ok {
		ch = NewmyChan()
		cm.key2Chan[key] = ch
	}

	ctx, cancle := context.WithTimeout(context.Background(), duration)
	defer cancle()

	cm.Unlock()

	select {
	case <-ch.ch:
		cm.Lock()
		defer cm.Unlock()
		return cm.conMap[key], nil
	case <-ctx.Done():
		return -1, ctx.Err()

	}

}
