// lrucache_test.go
package cache_test

import (
	"fmt"
	"time"

	"errors"

	"codeberg.org/tiny-frameworks/nexutils/cache"
)

// Example_base zeigt die grundlegende Nutzung des Caches.
func Example_base() {
	// Cache mit Kapazität 3, TTL 50ms, Cleanup alle 20ms
	c := cache.New(3, 50*time.Millisecond, 20*time.Millisecond)
	defer c.StopCleanup()

	c.Set("A", 1)
	c.Set("B", 2)
	c.Set("C", 3)

	// Direkt nach dem Setzen → Werte abrufen
	fmt.Println("Direkt nach Setzen:")

	item, found := c.Get("A")
	fmt.Println("A:", item, found) // 1

	item, found = c.Get("B")
	fmt.Println("B:", item, found) // 2

	item, found = c.Get("C")
	fmt.Println("C:", item, found) // 3

	// 4 Sekunden warten, damit die Werte ablaufen
	time.Sleep(80 * time.Millisecond)

	// Nach Ablauf der TTL → Werte abrufen (Cleanup sollte alte Einträge entfernt haben)
	fmt.Println("\nNach Ablauf der TTL:")

	item, found = c.Get("A")
	fmt.Println("A:", item, found) // nil, weil abgelaufen

	item, found = c.Get("B")
	fmt.Println("B:", item, found) // nil, weil abgelaufen

	item, found = c.Get("C")
	fmt.Println("C:", item, found) // nil, weil abgelaufen

	// Neues Element hinzufügen → Cache leert sich automatisch
	c.Set("D", 4)
	fmt.Println("\nNach dem Hinzufügen von D:")

	item, found = c.Get("D")
	fmt.Println("D:", item, found) // ❌ nil, weil abgelaufen

	// Output:
	// Direkt nach Setzen:
	// A: 1 true
	// B: 2 true
	// C: 3 true
	//
	// Nach Ablauf der TTL:
	// A: <nil> false
	// B: <nil> false
	// C: <nil> false
	//
	// Nach dem Hinzufügen von D:
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

	// Nächster Zugriff holt aus Cache, Loader wird NICHT aufgerufen
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

	// Daten setzen
	c.Set("A", 1)
	c.Set("B", 2)
	c.Set("C", 3)

	// Cache speichern
	if err := c.SaveToFile("cache.json"); err != nil {
		fmt.Println("Error saving:", err)
	}

	// Neuen Cache laden
	newCache := cache.New(3, 10*time.Second, 2*time.Second)
	if err := newCache.LoadFromFile("cache.json"); err != nil {
		fmt.Println("Error loading:", err)
	}

	// Werte abrufen
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

	// Zähler für deterministisches Verhalten im Test
	callCount := 0

	// Loader, der beim 2. Aufruf simuliert, dass die DB unerreichbar ist
	loader := func() (any, error) {
		callCount++
		if callCount == 2 {
			return nil, errors.New("DB unreachable")
		}
		return fmt.Sprintf("Value loaded (Call %d)", callCount), nil
	}

	// 1. Aufruf: Wert wird erfolgreich aus dem Loader geladen und gecacht
	val, err := c.GetOrLoad("user:42", loader)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Result:", val)
	}

	// 2. Aufruf: Wert kommt direkt aus dem Cache (Loader wird gar nicht ausgeführt!)
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
