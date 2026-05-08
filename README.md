# Kinetic

[![Go Reference](https://pkg.go.dev/badge/github.com/kevinmatthe/kinetic.svg)](https://pkg.go.dev/github.com/kevinmatthe/kinetic)
[![Go Report Card](https://goreportcard.com/badge/github.com/kevinmatthe/kinetic)](https://goreportcard.com/report/github.com/kevinmatthe/kinetic)

**Kinetic** 是一个高性能、易用的 Go 异步工具库，提供 Future、协程池、重试、并发迭代等功能。

## 特性

- ⚡ **高性能** - 最少内存分配，优于现有同类库的性能表现
- 🎯 **极其易用** - 常见操作一行代码搞定，API 命名直觉化
- 📦 **零外部依赖** - 纯 Go 实现，无第三方依赖
- 🔒 **类型安全** - 完整泛型支持，无 any 断言
- 🛡️ **并发安全** - 零锁设计，利用 happens-before 语义保证安全

## 安装

```bash
go get github.com/kevinmatthe/kinetic
```

要求 Go 1.24.1 或更高版本。

## 快速开始

```go
package main

import (
    "fmt"
    "github.com/kevinmatthe/kinetic"
)

func main() {
    // 并发启动多个异步任务
    users := kinetic.Go(func() ([]string, error) {
        return []string{"Alice", "Bob"}, nil
    })
    
    orders := kinetic.Go(func() (int, error) {
        return 42, nil
    })
    
    // 声明依赖 - users 和 orders 完成后才执行
    report := kinetic.Go(func() (string, error) {
        return fmt.Sprintf("Users: %v, Orders: %d", users.Val(), orders.Val()), nil
    }, kinetic.After(users, orders))
    
    // 获取结果
    r, err := report.Get()
    fmt.Println(r, err) // 输出: Users: [Alice Bob], Orders: 42 <nil>
}
```

## 核心功能

### 1. Future[T] - 异步计算的惰性句柄

Future 是异步操作的 handle，创建时不阻塞，取值时才等待。

```go
// 基础用法
a := kinetic.Go(func() (User, error) { return fetchUser() })

// 带依赖：等 a、b 完成后才执行，回调内 .Val() 安全
c := kinetic.Go(func() (Result, error) {
    return combine(a.Val(), b.Val()), nil
}, kinetic.After(a, b))

// 带上下文：支持取消/超时
d := kinetic.Go(func() (Result, error) {
    return fetchWithContext(ctx)
}, kinetic.WithContext(ctx))

// 取值方式
val := a.Val()           // 等待并返回值，错误时 panic
val, err := a.Get()      // 等待并返回值和错误（标准 Go 风格）
err := a.Err()           // 只返回错误

// 批量等待
kinetic.WaitAll(a, b, c)
```

### 2. 链式操作

```go
// Then - 转换成功值
name := kinetic.Then(userFuture, func(u User) (string, error) {
    return u.Name, nil
})

// Catch - 错误恢复
result := kinetic.Catch(userFuture, func(err error) (User, error) {
    return defaultUser, nil
})

// Finally - 清理（无论成功失败）
result := kinetic.Finally(userFuture, func() { cleanup() })
```

### 3. 组合器

```go
// All - 全部成功才成功，一个失败则立即失败
results, err := kinetic.All(a, b, c).Get()

// Race - 第一个完成（无论成败）即返回
result, err := kinetic.Race(a, b, c).Get()

// AllSettled - 等全部完成，不管成败
outcomes := kinetic.AllSettled(a, b, c).Val()

// Any - 第一个成功的结果
result, err := kinetic.Any(a, b, c).Get()
```

### 4. 协程池

```go
// Pool - 并发受限的任务池
p := kinetic.NewPool(10)  // 最多 10 个并发
for _, item := range items {
    p.Go(func() { process(item) })
}
p.Wait()  // 等待全部完成

// ResultPool - 收集结果的池
rp := kinetic.NewResultPool[string](10)
for _, item := range items {
    rp.Go(func() string { return transform(item) })
}
results := rp.Wait()  // []string

// ErrorPool - 收集错误的池
ep := kinetic.NewErrorPool(10)
for _, item := range items {
    ep.Go(func() error { return process(item) })
}
err := ep.Wait()  // 合并所有 error
```

### 5. 重试机制

```go
// 基础重试
result := kinetic.Retry(func() (string, error) {
    return unreliableAPI()
}, kinetic.WithMaxAttempts(3))

// 指数退避
result := kinetic.Retry(fn,
    kinetic.WithMaxAttempts(5),
    kinetic.WithExponentialBackoff(time.Second, 2.0),
    kinetic.WithMaxDelay(30*time.Second),
    kinetic.WithJitter(),
)

// 条件重试
result := kinetic.Retry(fn,
    kinetic.WithMaxAttempts(3),
    kinetic.WithRetryIf(func(err error) bool {
        return isRetryable(err)
    }),
)
```

### 6. 并发迭代

```go
// Map - 并发映射
results, err := kinetic.Map(items, func(item Item) (Result, error) {
    return process(item)
}, kinetic.WithConcurrency(10))

// ForEach - 并发遍历
err := kinetic.ForEach(items, func(item Item) error {
    return process(item)
})

// Filter - 并发过滤
filtered, err := kinetic.Filter(items, func(item Item) (bool, error) {
    return isValid(item)
})
```

### 7. Stream（有序并行处理）

```go
s := kinetic.NewStream[Input, Output](10)  // 最多 10 并发
for _, item := range items {
    s.Go(item, func(item Input) (Output, error) {
        return process(item)
    })
}
results, err := s.Wait()  // 保持输入顺序
```

### 8. 管道

```go
// FanIn - 多个 channel 合并
merged := kinetic.FanIn(ch1, ch2, ch3)

// FanOut - 一个 channel 分发到 N 个 worker
err := kinetic.FanOut(src, 5, func(item T) error { ... })

// Pipe - channel 间转换
outCh := kinetic.Pipe(inCh, 10, func(item T) (R, error) { ... })
```

### 9. Debounce / Throttle

```go
// Debounce - 延迟执行，最后一次调用生效
debounced := kinetic.Debounce(fn, 300*time.Millisecond)

// Throttle - 限速，固定间隔执行
throttled := kinetic.Throttle(fn, time.Second)
```

### 10. Once / Lazy

```go
// Once - 只执行一次，并发安全
o := kinetic.NewOnce(func() (Config, error) { return loadConfig() })
cfg, err := o.Get()  // 后续调用返回缓存结果

// Lazy - 延迟到首次 Get() 才执行
l := kinetic.NewLazy(func() (Connection, error) { return connect() })
conn, err := l.Get()
```

### 11. 信号量

```go
sem := kinetic.NewSemaphore(5)  // 最多 5 个并发
sem.Acquire(ctx)
defer sem.Release()
```

## 性能

采用零锁设计，利用 Go 的 happens-before 语义保证安全：

- `Go() + Get()`: < 500 ns/op, 2 allocs
- `Then` (已完成 Future): < 50 ns/op, 0 extra alloc
- 已完成的 Future 读取消耗: ~13ns (仅一次 channel recv)

详细性能数据请查看 `bench_test.go`。

## 设计原则

1. **单个 Future 无意义** - 异步的价值在于并发，先并发启动，再批量等待
2. **取值必须安全** - 不暴露公开字段，通过方法取值，防止未完成就读到零值
3. **依赖声明式** - Future 启动时可声明依赖，依赖全部完成后才执行，回调内 `.Val()` 安全

## License

MIT License - 详见 [LICENSE](LICENSE) 文件