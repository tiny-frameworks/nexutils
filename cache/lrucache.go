// Copyright 2026 Georg Hagn (tiny-frameworks)
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"container/list"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// CacheEntry stores key, value, and expiry time
type CacheEntry struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
}

// LRUCache is mainstructure
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	list     *list.List
	mu       sync.Mutex
	ttl      time.Duration
	stopCh   chan struct{}
}

// New creates a new LRU cache
func New(capacity int, ttl time.Duration, cleanupInterval time.Duration) *LRUCache {
	cache := &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
	go cache.startCleanup(cleanupInterval)
	return cache
}

// ---------------------- Basic Operations ----------------------

// Get retrieves a value or false if nothing is found or the date has expired.
func (c *LRUCache) Get(key string) (interface{}, bool) {

	c.mu.Lock()
	defer c.mu.Unlock()

	if element, found := c.cache[key]; found {
		entry := element.Value.(*CacheEntry)
		if time.Now().After(entry.ExpiresAt) {
			c.removeElement(element)
			return nil, false
		}
		c.list.MoveToFront(element)
		return entry.Value, true
	}

	return nil, false

}

// Set stores a value in the cache
func (c *LRUCache) Set(key string, value interface{}) {

	c.mu.Lock()
	defer c.mu.Unlock()

	if element, found := c.cache[key]; found {
		entry := element.Value.(*CacheEntry)
		entry.Value = value
		entry.ExpiresAt = time.Now().Add(c.ttl)
		c.list.MoveToFront(element)
		return
	}

	if c.list.Len() >= c.capacity {
		c.ejectOldest()
	}

	entry := &CacheEntry{Key: key, Value: value, ExpiresAt: time.Now().Add(c.ttl)}
	element := c.list.PushFront(entry)
	c.cache[key] = element

}

// ---------------------- Extensions ----------------------

// GetOrLoad: Retrieves a value from the cache or calls the loader.
// Only successful loader results are saved.
func (c *LRUCache) GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error) {

	c.mu.Lock()
	if element, found := c.cache[key]; found {
		entry := element.Value.(*CacheEntry)
		if time.Now().After(entry.ExpiresAt) {
			c.removeElement(element)
		} else {
			c.list.MoveToFront(element)
			val := entry.Value
			c.mu.Unlock()
			return val, nil
		}
	}
	c.mu.Unlock()

	val, err := loader()
	if err != nil {
		return nil, err
	}

	c.Set(key, val)
	return val, nil

}

// GetOrLoadWithFallback: like GetOrLoad, but provides a fallback in case of error
func (c *LRUCache) GetOrLoadWithFallback(
	key string,
	loader func() (interface{}, error),
	fallback interface{},
) (interface{}, error) {

	c.mu.Lock()
	if element, found := c.cache[key]; found {
		entry := element.Value.(*CacheEntry)
		if time.Now().After(entry.ExpiresAt) {
			c.removeElement(element)
		} else {
			c.list.MoveToFront(element)
			val := entry.Value
			c.mu.Unlock()
			return val, nil
		}
	}
	c.mu.Unlock()

	val, err := loader()
	if err != nil {
		return fallback, err
	}

	c.Set(key, val)
	return val, nil

}

// ---------------------- Persistence ----------------------

// SaveToFile stores the cache as JSON
func (c *LRUCache) SaveToFile(filename string) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var entries []CacheEntry
	for element := c.list.Front(); element != nil; element = element.Next() {
		entry := element.Value.(*CacheEntry)
		entries = append(entries, *entry)
	}

	return json.NewEncoder(file).Encode(entries)

}

// LoadFromFile loads cache content from JSON file
func (c *LRUCache) LoadFromFile(filename string) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var entries []CacheEntry
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		return err
	}

	c.cache = make(map[string]*list.Element)
	c.list = list.New()

	for _, entry := range entries {
		if time.Now().Before(entry.ExpiresAt) {
			element := c.list.PushFront(&entry)
			c.cache[entry.Key] = element
		}
	}
	return nil

}

// ---------------------- Background cleanup ----------------------

func (c *LRUCache) startCleanup(interval time.Duration) {

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpiredEntries()
		case <-c.stopCh:
			return
		}
	}

}

func (c *LRUCache) cleanupExpiredEntries() {

	c.mu.Lock()
	defer c.mu.Unlock()
	for element := c.list.Back(); element != nil; {
		entry := element.Value.(*CacheEntry)
		prev := element.Prev()
		if time.Now().After(entry.ExpiresAt) {
			c.removeElement(element)
		}
		element = prev
	}

}

// StopCleanup ends the cleanup routine.
func (c *LRUCache) StopCleanup() {
	close(c.stopCh)
}

// ---------------------- Helpers ----------------------

func (c *LRUCache) removeElement(element *list.Element) {
	entry := element.Value.(*CacheEntry)
	delete(c.cache, entry.Key)
	c.list.Remove(element)
}

func (c *LRUCache) ejectOldest() {
	oldest := c.list.Back()
	if oldest != nil {
		c.removeElement(oldest)
	}
}
