# Go Base

用于学习和实践 Go 基础能力的小型代码仓库。目前包含并发容器、带过期时间的互斥锁和 Redis 分布式锁实现。

## 环境要求

- Go 1.25.0 或更高版本

## 项目结构

```text
.
├── concurrentMap/  # 支持并发读写与等待机制的泛型 Map
├── expireLock/     # 支持自动过期释放的互斥锁
├── redislock/      # Redis 分布式锁
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
REDIS_ADDR=127.0.0.1:6379 go test ./redislock
```

## Redis 分布式锁

`redislock` 提供单节点 Redis 锁和基于独立 Redis 主节点的 RedLock。每次获取都会返回独立租约，使用新的 UUID v4 令牌；`Unlock` 只会释放该租约持有的锁。

`TryLock` 只尝试一次，竞争失败时返回 `redislock.ErrLocked`。`Lock` 按间隔重试，直到成功或传入的 context 结束。默认租约为 30 秒；自动续期需要显式指定 `WithAutoRenew()`。

```go
package main

import (
	"context"
	"time"

	"gobase/redislock"
	"github.com/gomodule/redigo/redis"
)

func main() {
	pool := &redis.Pool{
		DialContext: func(ctx context.Context) (redis.Conn, error) {
			return redis.DialContext(ctx, "tcp", "127.0.0.1:6379")
		},
	}
	defer pool.Close()

	client, err := redislock.NewClient(pool)
	if err != nil {
		panic(err)
	}
	lock, err := redislock.New("daily-job", client, redislock.WithTTL(30*time.Second))
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lease, err := lock.Lock(ctx)
	if err != nil {
		panic(err)
	}

	// 在租约有效期间执行受保护的工作。
	if err := lease.Unlock(context.Background()); err != nil {
		panic(err)
	}
}
```

获取锁用的 context 只限制获取过程，不控制成功后的租约。固定租约到期，或自动续期失败时，`lease.Done()` 会关闭。解锁应使用仍可用的 context。长时间运行的工作即使持有锁，也需要处理租约失效；对强一致写入场景还应在被保护资源侧使用 fencing token 等机制。

RedLock 使用 `NewRedLock(key, clients, ttl, nodeTimeout)` 创建，`clients` 应连接到至少三个相互独立的 Redis 主节点。它使用固定 TTL，不自动续期；获取失败会尝试释放各节点上的部分锁。

发布到 GitHub 前，需要将 `go.mod` 的 `gobase` 替换为最终仓库的模块路径，并同步修改 README 中的导入路径。
