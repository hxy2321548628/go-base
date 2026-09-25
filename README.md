# Go Base

用于学习和实践 Go 基础能力的小型代码仓库。目前包含并发容器与带过期时间的互斥锁实现。

## 环境要求

- Go 1.25.0 或更高版本

## 项目结构

```text
.
├── concurrentMap/  # 支持并发读写与等待机制的泛型 Map
├── expireLock/     # 支持自动过期释放的互斥锁
└── go.mod
```

## ConcurrentMap

`concurrentMap` 提供线程安全的泛型键值存储：

- `Set` 写入或覆盖指定 key 的值；
- `Get` 在 key 已存在时立即返回；
- key 不存在时，`Get` 会等待其他 goroutine 写入，直到成功或超时；
- 多个等待同一 key 的 goroutine 会在写入发生后同时被唤醒；
- 等待超时时返回值类型的零值以及 `context.DeadlineExceeded`。

```go
package main

import (
	"fmt"
	"time"

	concurrentmap "gobase/concurrentMap"
)

func main() {
	m := concurrentmap.NewConcurrentMap[string, int]()

	go func() {
		time.Sleep(100 * time.Millisecond)
		m.Set("answer", 42)
	}()

	value, err := m.Get("answer", time.Second)
	if err != nil {
		panic(err)
	}

	fmt.Println(value) // 42
}
```

## ExpireLock

`expireLock` 提供带自动过期能力的互斥锁。`Lock` 成功后返回与本次加锁绑定的解锁函数：

- `expire > 0` 时，到期后自动释放锁；
- `expire <= 0` 时，不自动释放，必须主动调用解锁函数；
- 重复解锁，或在锁已过期后解锁，会返回 `expirelock.ErrNotHolding`。

```go
package main

import (
	"fmt"
	"time"

	"gobase/expireLock"
)

func main() {
	var lock expirelock.ExpireLockS
	unlock := lock.Lock(time.Second)

	// 执行受保护的操作。

	if err := unlock(); err != nil {
		fmt.Println(err)
	}
}
```

## 运行测试

```bash
go test ./...
go test -race ./...
```
