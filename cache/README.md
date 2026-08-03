## nexutils/cache
<sup>the *cache module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

The `cache` module implements a lightweight, thread-safe **Least Recently Used (LRU)** cache for Go, featuring Time-To-Live (TTL) expiration, background cleanup, and JSON persistence.

---

## Features

* **LRU Strategy:** Automatically evicts the least recently used items when capacity is reached.
* **TTL Support:** Entries expire automatically after a defined duration.
* **Thread-Safe:** Safe for concurrent use via `sync.Mutex`.
* **Loader Pattern:** Simplifies data fetching with `GetOrLoad` and fallback options.
* **Persistence:** Save and restore your cache state to/from JSON files.
* **Background Cleanup:** Active goroutine to prune expired entries.

---

## Installation

```bash
go get codeberg.org/tiny-frameworks/nexutils/cache

```

---

## Quick Start

```go
package main

import (
	"fmt"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/cache"
)

func main() {
	// Initialize: capacity 100, 10m TTL, cleanup every 1m
	c := cache.New(100, 10*time.Minute, 1*time.Minute)
	defer c.StopCleanup()

	// Set a value
	c.Set("user_1", "Alice")

	// Get a value
	if val, found := c.Get("user_1"); found {
		fmt.Printf("Found: %v\n", val)
	}
}

```

---

## Extensions

### Lazy Loading (GetOrLoad)

Instead of checking for existence manually, provide a loader function. The cache handles the fetching and storage logic automatically:

```go
val, err := c.GetOrLoad("api_data", func() (interface{}, error) {
	return fetchDataFromRemoteAPI()
})

```

### Persistence

Easily persist your cache to disk to survive application restarts:

```go
// Save to file
c.SaveToFile("backup.json")

// Load from file (only non-expired items are restored)
c.LoadFromFile("backup.json")

```

---

## API Reference

| Method | Description |
| --- | --- |
| `New(cap, ttl, interval)` | Creates a new cache with capacity, TTL, and cleanup interval. |
| `Get(key)` | Returns the value and updates its LRU position. |
| `Set(key, value)` | Saves a value and resets its TTL. |
| `GetOrLoad(key, loader)` | Retrieves the value or loads it if missing using the `loader` function. |
| `SaveToFile(path)` | Exports the cache contents to a JSON file. |
| `LoadFromFile(path)` | Imports cache contents (only non-expired entries are kept). |
| `StopCleanup()` | Stops the background cleanup goroutine gracefully. |

---

## Examples

The `example_test.go` contains runnable implementations covering key scenarios:

1. **Basic:** Standard `Get` and `Set` operations.
2. **Lazy Loading:** Using `GetOrLoad` to fetch missing data.
3. **Persistence:** Demonstrating `SaveToFile` and `LoadFromFile`.
4. **TTL & Cleanup:** Showcasing how the background cleaner works.

Run all tests and examples via:

```bash
go test -v ./...

```

---

## How it works (LRU & TTL)

The cache combines a **hash map** for $O(1)$ fast access with a **doubly linked list** to track usage order.

* **Read access:** An item is moved to the head of the list.
* **Write access:** New items are pushed to the head; when capacity is reached, the tail item (least recently used) is evicted.
* **Expiration:** The background routine checks for expired timestamps at defined intervals to efficiently free up memory.

---

## Best Practices

### Choosing a Cleanup Interval

* **Frequent (e.g., 10s):** Ideal for small caches with high turnover where the memory footprint is critical.
* **Balanced (e.g., 1m – 5m):** Recommended for most standard use cases.
* **Passive (e.g., 1h):** Sufficient if the cache is large and expired items are likely to be evicted by the LRU logic anyway.

### Type Assertions

Since the cache stores `interface{}` (or `any`), always use type assertions when retrieving values:

```go
if val, found := c.Get("myKey"); found {
	data := val.(string) // Assert to your expected type
}

```

---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Namespace:** `codeberg.org/tiny-frameworks/nexutils/cache`
* **License:** Apache License, Version 2.0.

*GSF-nexutils/cache is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact

If you have questions or feedback, feel free to reach out:

📧 *georghagn [at] tiny-frameworks.io*

---
