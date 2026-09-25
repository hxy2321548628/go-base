package expirelock

// 实现一个带过期事件的互斥锁

import (
	"errors"
	"sync"
	"time"
)

type ExpireLockS struct {
	mutex   sync.Mutex // 真正的锁
	mu      sync.Mutex // 保护下面的字段
	gen     uint64     // 当前这次持有的代次, 0 表示无人持有
	nextGen uint64     // 代次分配器, 唯一token
	timer   *time.Timer
}

var ErrNotHolding = errors.New("not your lock!!!")

// 返回解锁函数, token 藏在闭包里, 调用方不用管
func (e *ExpireLockS) Lock(expire time.Duration) (unlockFunc func() error) {
	e.mutex.Lock()

	e.mu.Lock()
	defer e.mu.Unlock()

	e.nextGen++
	e.gen = e.nextGen
	gen := e.gen

	if expire > 0 {
		// AfterFunc 省掉一个协程和一个 context, Stop() 就是取消
		e.timer = time.AfterFunc(expire, func() { e.unlock(gen) })
	}
	return func() error { return e.unlock(gen) }
}

func (e *ExpireLockS) unlock(gen uint64) error {
	// 必须使用两把锁, 否则在解锁时就会出现自己锁自己的死锁情况
	e.mu.Lock()
	defer e.mu.Unlock()

	// 代次对不上: 要么已经过期自动释放了, 要么已经解过一次了
	if e.gen != gen {
		return ErrNotHolding
	}
	e.gen = 0 // 让这个 token 立即失效, 杜绝重复解锁

	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}
	e.mutex.Unlock()
	return nil
}
