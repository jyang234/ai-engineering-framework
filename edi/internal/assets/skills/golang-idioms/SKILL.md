---
name: golang-idioms
description: Go language idioms from Effective Go
languages: [go]
---

# Go Idioms

Idiomatic patterns from Effective Go. Apply these when writing or reviewing Go code.

---

## Zero Values

Go types have useful zero values. Design structs so zero values are valid.

```go
// ✅ Buffer is immediately usable without initialization
var buf bytes.Buffer
buf.WriteString("hello")

// ✅ Mutex zero value is unlocked and ready
type SafeCounter struct {
    mu    sync.Mutex  // Zero value is unlocked
    count int
}

// ✅ Slice zero value (nil) works with append
var items []string
items = append(items, "first")  // Works fine
```

**Design principle:** If a zero-value struct isn't useful, provide a constructor.

---

## Embedding

Use embedding for composition, not inheritance. Embedded types promote their methods.

```go
// Embed for behavior composition
type Logger struct {
    *log.Logger
    prefix string
}

// Methods from log.Logger are promoted
logger := &Logger{Logger: log.New(os.Stdout, "", 0)}
logger.Println("promoted method")  // Calls embedded Logger.Println

// Embed interfaces to satisfy them
type ReadWriter struct {
    io.Reader
    io.Writer
}
```

**Caution:** Embedding exposes all methods. Don't embed if you want encapsulation.

---

## Getters and Setters

Go doesn't use `Get` prefix for getters. Use `Set` prefix only for setters.

```go
// ✅ Idiomatic
func (u *User) Name() string     { return u.name }     // Getter: no "Get"
func (u *User) SetName(n string) { u.name = n }        // Setter: uses "Set"

// ❌ Non-idiomatic
func (u *User) GetName() string { return u.name }
```

---

## Interface Naming

Single-method interfaces use `-er` suffix. Multi-method interfaces describe the role.

```go
// Single method → -er suffix
type Reader interface { Read(p []byte) (n int, err error) }
type Stringer interface { String() string }
type Handler interface { ServeHTTP(ResponseWriter, *Request) }

// Multi-method → descriptive role name
type ReadWriteCloser interface {
    Read(p []byte) (n int, err error)
    Write(p []byte) (n int, err error)
    Close() error
}
```

---

## Defer

Use `defer` for cleanup. Deferred calls execute LIFO when the function returns.

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()  // Guaranteed cleanup

    // Work with file...
    return nil
}
```

**Gotchas:**
```go
// Defer evaluates arguments immediately
for _, f := range files {
    defer f.Close()  // ⚠️ All close same file (last value of f)
}

// Fix: capture in closure or use immediate variable
for _, f := range files {
    f := f  // Shadow variable
    defer f.Close()
}
```

---

## Type Switches

Use type switches for interface type assertions.

```go
func describe(i interface{}) string {
    switch v := i.(type) {
    case nil:
        return "nil"
    case int:
        return fmt.Sprintf("int: %d", v)
    case string:
        return fmt.Sprintf("string: %q", v)
    case fmt.Stringer:
        return fmt.Sprintf("Stringer: %s", v.String())
    default:
        return fmt.Sprintf("unknown: %T", v)
    }
}
```

**Prefer type switches over chains of type assertions.**

---

## Blank Identifier

Use `_` to discard unwanted values or satisfy interfaces.

```go
// Discard unwanted return values
_, err := io.Copy(dst, src)

// Import for side effects only
import _ "image/png"  // Register PNG decoder

// Compile-time interface check
var _ io.Reader = (*MyReader)(nil)
```

---

## Channel Idioms

### Signaling completion
```go
done := make(chan struct{})
go func() {
    // work...
    close(done)  // Signal completion
}()
<-done  // Wait for completion
```

### Fan-out, fan-in
```go
func fanOut(in <-chan int, workers int) []<-chan int {
    outs := make([]<-chan int, workers)
    for i := 0; i < workers; i++ {
        outs[i] = worker(in)
    }
    return outs
}
```

### Select with timeout
```go
select {
case result := <-ch:
    handle(result)
case <-time.After(5 * time.Second):
    return errors.New("timeout")
case <-ctx.Done():
    return ctx.Err()
}
```

---

## Context Usage

Always pass context as the first parameter. Never store it in a struct.

```go
// ✅ Correct
func (s *Service) Process(ctx context.Context, req Request) error {
    // Use ctx for cancellation and deadlines
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    // ...
}

// ❌ Never store context
type Service struct {
    ctx context.Context  // Don't do this
}
```

**Propagate context through call chains. Don't create new contexts mid-chain.**

---

## Initialization

### Package-level variables
```go
var (
    mu      sync.Mutex
    clients = make(map[string]*Client)
)
```

### init() functions
Use sparingly. Prefer explicit initialization.

```go
// ✅ Acceptable: register with global registry
func init() {
    sql.Register("postgres", &Driver{})
}

// ❌ Avoid: complex setup with side effects
func init() {
    conn, _ := net.Dial("tcp", "db:5432")  // Don't do this
}
```

---

## Make vs New

- `new(T)` returns `*T`, zeroed
- `make(T, args)` returns initialized `T` (slices, maps, channels only)

```go
// new: allocates zeroed storage, returns pointer
p := new(int)      // *int, points to 0
s := new(MyStruct) // *MyStruct, all fields zero

// make: creates initialized internal structure
slice := make([]int, 10, 100)  // len=10, cap=100
ch := make(chan int, 5)        // buffered channel
m := make(map[string]int)      // ready-to-use map
```

---

## Variadic Functions

Use `...` for variable arguments. Useful for optional parameters.

```go
func Printf(format string, args ...interface{}) {
    // args is []interface{}
}

// Passing slice to variadic
values := []int{1, 2, 3}
sum(values...)  // Expand slice
```

---

## Method Receivers

| Receiver | Use when |
|----------|----------|
| Value `(t T)` | Small, immutable, or copy is intended |
| Pointer `(t *T)` | Mutates receiver, large struct, or consistency with other methods |

```go
// Value receiver: doesn't modify, small type
func (p Point) Distance(q Point) float64 {
    return math.Hypot(q.X-p.X, q.Y-p.Y)
}

// Pointer receiver: modifies state
func (p *Point) ScaleBy(factor float64) {
    p.X *= factor
    p.Y *= factor
}
```

**Consistency rule:** If any method needs pointer receiver, all methods should use pointer receiver.

---

## Constants and iota

```go
type ByteSize float64

const (
    _           = iota  // ignore first value
    KB ByteSize = 1 << (10 * iota)
    MB
    GB
    TB
)

// Typed constants for type safety
type Status int

const (
    StatusPending Status = iota
    StatusActive
    StatusComplete
)
```

---

## Avoiding Common Mistakes

### Loop variable capture
```go
// ❌ Bug: all goroutines share same variable
for _, item := range items {
    go func() {
        process(item)  // Uses final value of item
    }()
}

// ✅ Fix: pass as parameter or shadow
for _, item := range items {
    item := item  // Shadow
    go func() {
        process(item)
    }()
}
```

### Nil interface vs nil pointer
```go
var p *bytes.Buffer = nil
var i io.Writer = p

// ⚠️ i is NOT nil! It's an interface holding a nil pointer
if i != nil {
    i.Write([]byte("oops"))  // Panic
}
```

### Slice gotchas
```go
// Slices share underlying array
a := []int{1, 2, 3, 4, 5}
b := a[1:3]
b[0] = 99  // Also modifies a[1]

// Use copy for independence
b := make([]int, 2)
copy(b, a[1:3])
