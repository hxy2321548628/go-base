package concurrentmap

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestConcurrentMapGetExisting(t *testing.T) {
	// 已存在的 key 应立即返回；即使超时时间为 0，也不应产生超时错误。
	cm := NewConcurrentMap()
	cm.Set(1, 100)

	got, err := cm.Get(1, 0)
	if err != nil {
		t.Fatalf("Get() returned an unexpected error: %v", err)
	}
	if got != 100 {
		t.Fatalf("Get() = %d, want 100", got)
	}
}

func TestConcurrentMapSetOverwritesValue(t *testing.T) {
	// 对同一个 key 再次赋值后，Get 应返回最后一次写入的值。
	cm := NewConcurrentMap()
	cm.Set(1, 100)
	cm.Set(1, 200)

	got, err := cm.Get(1, time.Second)
	if err != nil {
		t.Fatalf("Get() returned an unexpected error: %v", err)
	}
	if got != 200 {
		t.Fatalf("Get() = %d, want 200", got)
	}
}

func TestConcurrentMapGetTimeout(t *testing.T) {
	// key 在指定时间内没有写入时，Get 应返回约定值和超时错误。
	cm := NewConcurrentMap()

	got, err := cm.Get(1, 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Get() error = %v, want context.DeadlineExceeded", err)
	}
	if got != -1 {
		t.Fatalf("Get() = %d, want -1", got)
	}
}

func TestConcurrentMapGetWaitsForSet(t *testing.T) {
	// key 不存在时 Get 应阻塞，随后 Set 应唤醒 Get 并传回写入的值。
	cm := NewConcurrentMap()
	result := make(chan int, 1)
	errResult := make(chan error, 1)

	go func() {
		got, err := cm.Get(1, time.Second)
		result <- got
		errResult <- err
	}()

	// 确认 Get 已进入等待分支，避免 Set 抢先执行而未覆盖唤醒逻辑。
	waitUntilWaiting(t, cm, 1)
	cm.Set(1, 100)

	select {
	case got := <-result:
		if got != 100 {
			t.Fatalf("Get() = %d, want 100", got)
		}
	case <-time.After(time.Second):
		t.Fatal("Get() was not released by Set()")
	}

	if err := <-errResult; err != nil {
		t.Fatalf("Get() returned an unexpected error: %v", err)
	}
}

func TestConcurrentMapMultipleWaiters(t *testing.T) {
	// 多个 goroutine 同时读取同一个缺失的 key 时，都应安全地得到写入值。
	const waiterCount = 100

	cm := NewConcurrentMap()
	results := make(chan int, waiterCount)
	errs := make(chan error, waiterCount)
	var started sync.WaitGroup
	started.Add(waiterCount)

	for i := 0; i < waiterCount; i++ {
		go func() {
			started.Done()
			got, err := cm.Get(1, time.Second)
			results <- got
			errs <- err
		}()
	}

	// 所有 goroutine 均已启动，并确保至少一个 Get 已注册等待通道后再写入。
	started.Wait()
	waitUntilWaiting(t, cm, 1)
	cm.Set(1, 100)

	for i := 0; i < waiterCount; i++ {
		if err := <-errs; err != nil {
			t.Errorf("Get() returned an unexpected error: %v", err)
		}
		if got := <-results; got != 100 {
			t.Errorf("Get() = %d, want 100", got)
		}
	}
}

func TestConcurrentMapConcurrentDistinctKeys(t *testing.T) {
	// 并发操作不同 key 时，每个 Get 都应读取到对应 key 的值且不发生数据竞争。
	const keyCount = 100

	cm := NewConcurrentMap()
	var wg sync.WaitGroup
	wg.Add(keyCount)

	for key := 0; key < keyCount; key++ {
		key := key
		go func() {
			defer wg.Done()

			cm.Set(key, key*10)
			got, err := cm.Get(key, time.Second)
			if err != nil {
				t.Errorf("Get(%d) returned an unexpected error: %v", key, err)
				return
			}
			if want := key * 10; got != want {
				t.Errorf("Get(%d) = %d, want %d", key, got, want)
			}
		}()
	}

	wg.Wait()
}

func waitUntilWaiting(t *testing.T, cm *ConcurrentMap, key int) {
	// 轮询内部等待通道仅用于测试同步，不依赖固定休眠时间判断 goroutine 状态。
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		cm.Lock()
		_, ok := cm.key2Chan[key]
		cm.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatalf("Get(%d) did not start waiting", key)
}
