// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"fmt"
	"time"

	"errors"

	"codeberg.org/tiny-frameworks/nexutils/cache"
)

// Example_base demonstrates the basic usage of the cache.
func Example_base() {
	// Cache with Capazity 3, TTL 50ms, Cleanup every 20ms
	c := cache.New(3, 50*time.Millisecond, 20*time.Millisecond)
	defer c.StopCleanup()

	c.Set("A", 1)
	c.Set("B", 2)
	c.Set("C", 3)

	// Immediately after setting → Retrieve values
	fmt.Println("Immediately after setting:")

	item, found := c.Get("A")
	fmt.Println("A:", item, found) // 1

	item, found = c.Get("B")
	fmt.Println("B:", item, found) // 2

	item, found = c.Get("C")
	fmt.Println("C:", item, found) // 3

	// Wait 4 seconds for the values ​​to expire.
	time.Sleep(80 * time.Millisecond)

	// After TTL expiration → retrieve values ​​(cleanup should have removed old entries)
	fmt.Println("\nAfter the TTL expires:")

	item, found = c.Get("A")
	fmt.Println("A:", item, found) // nil, because expired

	item, found = c.Get("B")
	fmt.Println("B:", item, found) // nil, because expired

	item, found = c.Get("C")
	fmt.Println("C:", item, found) // nil, because expired

	// Add new element → Cache clears automatically
	c.Set("D", 4)
	fmt.Println("\nAfter adding D:")

	item, found = c.Get("D")
	fmt.Println("D:", item, found) // ❌ nil, because expired

	// Output:
	// Immediately after setting:
	// A: 1 true
	// B: 2 true
	// C: 3 true
	//
	// After the TTL expires:
	// A: <nil> false
	// B: <nil> false
	// C: <nil> false
	//
	// After adding D:
	// D: 4 true

}

func Example_komplett() {

	type cacheItem struct {
		ID    int
		Code  string
		Label string
	}

	// Cache with Capazity 3, TTL 5s, Cleanup alle 2s
	c := cache.New(3, 3*time.Second, 2*time.Second)
	defer c.StopCleanup()

	// -------- Simple Set/Get --------
	c.Set("foo", "bar")
	if val, ok := c.Get("foo"); ok {
		fmt.Println("Get foo:", val) // → "bar"
	}

	// ---complex Set/Get with struct --------
	item := &cacheItem{
		ID:    100,
		Code:  "CAB",
		Label: "Label for CAB 100",
	}

	c.Set("CAB", item)
	if val, ok := c.Get("CAB"); ok {
		fmt.Println("Get CAB:", val)
	}

	// -------- TTL Sequence Test --------
	c.Set("temp", "value")
	fmt.Println("Set temp: value")
	time.Sleep(4 * time.Second) // longer than TTL
	if _, ok := c.Get("temp"); !ok {
		fmt.Println("temp expired!")
	}

	// -------- GetOrLoad mit Loader --------
	val, err := c.GetOrLoad("user:1", func() (interface{}, error) {
		fmt.Println("Loader called for user:1")
		return "Alice", nil
	})
	fmt.Println("user:1 =", val, "err:", err)

	// Next access retrieves from cache; loader is NOT called.
	val, _ = c.GetOrLoad("user:1", func() (interface{}, error) {
		fmt.Println("This Loader should not run!")
		return "Bob", nil
	})
	fmt.Println("user:1 =", val)

	// -------- GetOrLoadWithFallback --------
	val, err = c.GetOrLoadWithFallback("user:2", func() (interface{}, error) {
		fmt.Println("Loader fail for user:2")
		return nil, fmt.Errorf("DB down")
	}, "FallbackUser")
	fmt.Println("user:2 =", val, "err:", err)

	// -------- Persistenz: Save/Load --------
	c.Set("session", "abc123")
	if err := c.SaveToFile("cache.json"); err != nil {
		fmt.Println("Error saving:", err)
	} else {
		fmt.Println("Cache in cache.json saved")
	}

	// Neuen Cache laden
	newCache := cache.New(3, 5*time.Second, 2*time.Second)
	defer newCache.StopCleanup()

	if err := newCache.LoadFromFile("cache.json"); err != nil {
		fmt.Println("Error loading:", err)
	} else if val, ok := newCache.Get("session"); ok {
		fmt.Println("Loaded value session:", val)
	}

	// Output:
	// Get foo: bar
	// Get CAB: &{100 CAB Label for CAB 100}
	// Set temp: value
	// temp expired!
	// Loader called for user:1
	// user:1 = Alice err: <nil>
	// user:1 = Alice
	// Loader fail for user:2
	// user:2 = FallbackUser err: DB down
	// Cache in cache.json saved
	// Loaded value session: abc123

}

func Example_persistencen() {
	c := cache.New(3, 10*time.Second, 2*time.Second)

	// Set data
	c.Set("A", 1)
	c.Set("B", 2)
	c.Set("C", 3)

	// save Cache
	if err := c.SaveToFile("cache.json"); err != nil {
		fmt.Println("Error saving:", err)
	}

	// load new cache
	newCache := cache.New(3, 10*time.Second, 2*time.Second)
	if err := newCache.LoadFromFile("cache.json"); err != nil {
		fmt.Println("Error loading:", err)
	}

	// Retrieve values
	item, found := c.Get("A")
	fmt.Println("A:", item, found) // 1
	item, found = c.Get("B")
	fmt.Println("B:", item, found) // 2
	item, found = c.Get("C")
	fmt.Println("C:", item, found) // 3

	// Output:
	// A: 1 true
	// B: 2 true
	// C: 3 true
}

func Example_getOrReload() {
	c := cache.New(3, 5*time.Second, 2*time.Second)
	defer c.StopCleanup()

	// Counter for deterministic behavior during testing
	callCount := 0

	// Loader that simulates the database being unreachable on the second call.
	loader := func() (any, error) {
		callCount++
		if callCount == 2 {
			return nil, errors.New("DB unreachable")
		}
		return fmt.Sprintf("Value loaded (Call %d)", callCount), nil
	}

	// 1st call: Value is successfully loaded from the loader and cached.
	val, err := c.GetOrLoad("user:42", loader)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Result:", val)
	}

	// 2nd call: Value comes directly from the cache (loader is not executed at all!)
	val, err = c.GetOrLoad("user:42", loader)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Result (from cache):", val)
	}

	// Output:
	// Result: Value loaded (Call 1)
	// Result (from cache): Value loaded (Call 1)
}
