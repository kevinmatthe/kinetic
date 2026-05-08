# Kinetic - Go 异步工具库实现方案

## 设计目标
- **高性能**: 最少内存分配, 优于现有同类库的 benchmark 数据
- **极其易用**: 常见操作一行代码搞定, API 命名直觉化
- **功能全面**: 覆盖所有常见异步范式
- **零外部依赖**: 纯 Go 实现
- **泛型优先**: 全类型安全, 无 any 断言

## 核心设计原则

1. **单独 `.Await()` 无意义** —— 异步的价值在于并发，一个 Future 等 一个结果等价于同步调用
2. **取值必须安全** —— 不暴露公开字段，通过方法取值，防止未完成就读到零值
3. **先并发启动，再批量等待，最后取值** —— 这是 Go 异步的核心范式
4. **依赖声明式** —— Future 启动时可声明依赖，依赖全部完成后才执行，回调内 `.Val()` 安全

## 项目结构
```
kinetic/
├── future.go          # Future[T] 核心类型
├── combine.go         # 组合器: All, Race, AllSettled, Any
├── pool.go            # 协程池: Pool, ResultPool, ErrorPool
├── retry.go           # 重试: 指数退避, 条件重试
├── iter.go            # 并发迭代: Map, ForEach, Filter
├── stream.go          # 有序流处理
├── pipe.go            # 管道: FanIn, FanOut, Pipe
├── debounce.go        # 防抖 & 节流
├── once.go            # Once & Lazy 单次求值
├── semaphore.go       # 信号量
├── future_test.go
├── combine_test.go
├── pool_test.go
├── retry_test.go
├── iter_test.go
├── stream_test.go
├── pipe_test.go
├── debounce_test.go
├── once_test.go
├── semaphore_test.go
└── bench_test.go      # 全量 benchmark
```

## 核心 API 设计

### 1. Future[T] - 异步计算的惰性句柄

**核心理念**: Future 是异步操作的 handle，创建时不阻塞，取值时才等待。

```go
// ========== 构造 ——— 统一入口 Go + Option 模式 ==========

// 基础：启动异步计算
a := kinetic.Go(func() (User, error) { return fetchUser() })

// 带依赖：等 a、b 完成后才执行，回调内 .Val() 安全
c := kinetic.Go(func() (Result, error) {
    return combine(a.Val(), b.Val()), nil
}, kinetic.After(a, b))

// 带上下文：ctx 取消时 Future 也取消
b := kinetic.Go(func() (User, error) { return fetchUser() },
    kinetic.WithContext(ctx))

// 带依赖 + 上下文：Option 自由组合
d := kinetic.Go(func() (Result, error) {
    return combine(a.Val(), b.Val()), nil
}, kinetic.After(a, b), kinetic.WithContext(ctx))

// ========== Option 列表 ==========
// kinetic.After(deps ...Awaitable)  — 等待依赖完成后再执行
// kinetic.WithContext(ctx)           — 绑定上下文，支持取消/超时

// ========== 等待 ==========

// WaitAll 批量等待所有 Future 完成
kinetic.WaitAll(a, b, c)

// Wait 等待单个 Future 完成
a.Wait()

// ========== 取值（方法，非字段）==========

// Val() 等待并返回值，如果有 error 则 panic（适合确定成功的场景）
users := a.Val()          // User

// Get() 等待并返回值和 error（标准 Go 风格）
users, err := a.Get()     // (User, error)

// Err() 等待并只返回 error
if err := a.Err(); err != nil { ... }

// ========== 典型使用模式 ==========

// 并发启动多个异步任务
a := kinetic.Go(fetchUsers)
b := kinetic.Go(fetchOrders)
c := kinetic.Go(fetchSettings)

// 声明依赖 —— a、b、c 都完成后才执行
d := kinetic.Go(func() (Dashboard, error) {
    return buildDashboard(a.Val(), b.Val(), c.Val()), nil
}, kinetic.After(a, b, c))

// 也可以手动 WaitAll + 取值
kinetic.WaitAll(a, b, c)
users, err := a.Get()
```

**为什么用 Option 模式**:
- 一个 `Go` 函数搞定所有场景，不搞 Go/GoC/GoAfter/GoAfterC 变体爆炸
- Option 自由组合，按需添加
- 未来新增能力（如优先级、超时）只需加 Option，不改签名
- Go 社区惯用模式，用户零学习成本

**为什么支持依赖参数**:
- 声明依赖后，回调内 `.Val()` 一定安全（依赖已完成）
- 库自动等待依赖，不需要手动 WaitAll
- 依赖关系显式可见，代码自文档化
- 形成天然的 DAG 执行图，无依赖的任务自动并发

**依赖的 Future 类型问题**:
Go 泛型不支持可变类型参数，依赖的 Future 类型不同（`*Future[User]`, `*Future[Order]`），
需要用 `Awaitable` 接口统一：

```go
// Awaitable 是 Future 的非泛型接口，用于依赖声明
type Awaitable interface {
    Wait()
}

// Future[T] 实现 Awaitable
func (f *Future[T]) Wait() { <-f.done }
```

这样 `GoAfter(fn, a, b)` 中 a、b 是不同类型的 `*Future[X]`，但都满足 `Awaitable`。

**为什么 Val/Get/Err 是方法而不是字段**:
- 如果是公开字段 `.Val`，未 Wait 就读取会拿到零值，编译器不报错，运行时是隐蔽 bug
- 方法可以在内部先确保完成再返回值，永远安全
- 还可以在 debug 模式下检测"未等待就取值"的误用

**内部实现**:
```go
type Future[T any] struct {
    done  chan struct{}  // close 完成时关闭, 零开销同步
    value T             // 写在 close 之前, happens-before 保证安全
    err   error
}

func (f *Future[T]) Get() (T, error) {
    <-f.done   // 等待完成 (已完成时零开销)
    return f.value, f.err
}

func (f *Future[T]) Val() T {
    val, err := f.Get()
    if err != nil {
        panic(err)  // 确定成功时使用的快捷方式
    }
    return val
}

func (f *Future[T]) Err() error {
    <-f.done
    return f.err
}
```

- 零锁设计: 利用 Go 的 happens-before 语义 (`close(ch)` happens-before `<-ch`)
- 仅 1 次 channel 分配
- 已完成的 Future 读取消耗: ~13ns (仅一次 channel recv)

### 2. 链式操作 (包级函数, 规避 Go 方法不支持独立类型参数)

```go
// Then - 转换成功值
name := kinetic.Then(a, func(u User) (string, error) {
    return u.Name, nil
})

// ThenC - 带上下文的转换
result := kinetic.ThenC(a, ctx, func(ctx context.Context, u User) (string, error) {
    return processWithContext(ctx, u)
})

// Catch - 错误恢复
result := kinetic.Catch(a, func(err error) (User, error) {
    return defaultUser, nil
})

// Finally - 清理 (无论成功失败)
result := kinetic.Finally(a, func() { cleanup() })
```

**优化**: `Then` 对已完成的 Future 走快速路径, 不启动额外 goroutine

### 3. 组合器

```go
// All - 全部成功才成功, 一个失败则立即失败
results := kinetic.All(a, b, c)   // *Future[[]T]
vals, err := results.Get()

// Race - 第一个完成(无论成败)即返回
result := kinetic.Race(a, b, c)   // *Future[T]
val, err := result.Get()

// AllSettled - 等全部完成, 不管成败
outcomes := kinetic.AllSettled(a, b, c)  // *Future[[]Result[T]]
vals := outcomes.Val()

// Any - 第一个成功的结果
result := kinetic.Any(a, b, c)    // *Future[T]

// Result 类型
type Result[T any] struct {
    Value T
    Error error
}
```

### 4. 协程池

```go
// Pool - 并发受限的任务池
p := kinetic.NewPool(10)  // 最多 10 个并发
for _, item := range items {
    p.Go(func() { process(item) })
}
p.Wait()  // 等待全部完成, 自动传播 panic

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

// ContextPool - 首个错误取消其余任务
cp := kinetic.NewContextPool(ctx, 10)
```

**Panic 处理**: 所有 Pool 自动 recover panic, 在 Wait() 时 re-panic 附带完整 stack trace

### 5. Retry

```go
// 基础重试
result := kinetic.Retry(func() (string, error) {
    return unreliableAPI()
}, kinetic.WithMaxAttempts(3))
val, err := result.Get()

// 指数退避
result := kinetic.Retry(fn,
    kinetic.WithMaxAttempts(5),
    kinetic.WithExponentialBackoff(time.Second, 2.0),
    kinetic.WithMaxDelay(30*time.Second),
    kinetic.WithJitter(),
)

// 条件重试 (仅对特定错误重试)
result := kinetic.Retry(fn,
    kinetic.WithMaxAttempts(3),
    kinetic.WithRetryIf(func(err error) bool {
        return isRetryable(err)
    }),
)

// 带上下文的重试
result := kinetic.RetryC(ctx, fn, opts...)
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

### 7. Stream (有序并行处理)

```go
s := kinetic.NewStream[Input, Output](10)  // 最多 10 并发
for _, item := range items {
    s.Go(item, func(item Input) (Output, error) {
        return process(item)
    })
}
results, err := s.Wait()  // 保持输入顺序
```

### 8. Pipe (管道)

```go
// FanIn - 多个 channel 合并
merged := kinetic.FanIn(ch1, ch2, ch3)  // <-chan T

// FanOut - 一个 channel 分发到 N 个 worker
err := kinetic.FanOut(src, 5, func(item T) error { ... })

// Pipe - channel 间转换
outCh := kinetic.Pipe(inCh, 10, func(item T) (R, error) { ... })
```

### 9. Debounce / Throttle

```go
// Debounce - 延迟执行, 最后一次调用生效
debounced := kinetic.Debounce(fn, 300*time.Millisecond)
for i := 0; i < 10; i++ {
    debounced()  // 只有最后一次会在 300ms 后执行
}

// Throttle - 限速, 固定间隔执行
throttled := kinetic.Throttle(fn, time.Second)
```

### 10. Once / Lazy

```go
// Once - 只执行一次, 并发安全
o := kinetic.NewOnce(func() (Config, error) { return loadConfig() })
cfg, err := o.Get()  // 首次调用执行 fn, 后续调用返回缓存结果

// Lazy - 延迟到首次 Get() 才执行
l := kinetic.NewLazy(func() (Connection, error) { return connect() })
conn, err := l.Get()
```

### 11. Semaphore

```go
sem := kinetic.NewSemaphore(5)  // 最多 5 个并发
sem.Acquire(ctx)
defer sem.Release()
```

## 性能目标 (Benchmark 约束)

| 操作 | 目标 | 对标 (kelindar/async) |
|------|------|----------------------|
| `Go() + Get()` | < 500 ns/op, 2 allocs | 507 ns/op, 2 allocs |
| `Go(deps...) + Get()` | < 600 ns/op, 2 allocs | - |
| `WaitAll(N=100)` | < 100μs/op | - |
| `Then` (已完成 Future) | < 50 ns/op, 0 extra alloc | - |
| `Pool.Go + Wait` | 接近 raw goroutine | - |
| `Map(N=1000, conc=10)` | - | - |

## 实现顺序

1. **Phase 1**: `future.go` + `combine.go` + 测试 + benchmark
2. **Phase 2**: `pool.go` + 测试 + benchmark
3. **Phase 3**: `retry.go` + 测试 + benchmark
4. **Phase 4**: `iter.go` + 测试 + benchmark
5. **Phase 5**: `stream.go` + `pipe.go` + 测试 + benchmark
6. **Phase 6**: `debounce.go` + `once.go` + `semaphore.go` + 测试
7. **Phase 7**: 全量 benchmark 优化
