package main

import (
	"sync"
	"time"
)

type Cache[K comparable, V any] interface {
	Get(K) V
	Put(K, V)
}

type CacheValue[V any] struct {
	Value      V
	Expiration time.Time
}

func (this *CacheValue[V]) Expired() bool {
	return this.Expiration.Before(time.Now())
}

type loadingCache[K comparable, V any] struct {
	dataMap       map[K]*CacheValue[V]
	lockMap       map[K]*sync.Mutex
	lockMapLock   *sync.Mutex
	loadingFunc   func(K) V
	cacheDuration time.Duration
}

func NewLoadingCache[K comparable, V any](loadingFunc func(K) V, cacheDuration time.Duration) Cache[K, V] {
	return &loadingCache[K, V]{
		dataMap:       map[K]*CacheValue[V]{},
		lockMap:       map[K]*sync.Mutex{},
		lockMapLock:   &sync.Mutex{},
		loadingFunc:   loadingFunc,
		cacheDuration: cacheDuration,
	}
}

func (l loadingCache[K, V]) Get(key K) V {
	keyLock := l.getKeyLockFromMap(key)

	// try to claim read lock
	keyLock.Lock()
	defer keyLock.Unlock()

	// if no item in the map load items, put and then return
	// only blocking operation in the map
	value, exists := l.dataMap[key]

	// if not exists load OR value does exist but is expired
	if !exists || (exists && value.Expired()) {
		newValue := l.loadingFunc(key)
		// put new data in map
		l.dataMap[key] = &CacheValue[V]{
			Value:      newValue,
			Expiration: time.Now().Add(l.cacheDuration),
		}
		return newValue
	} else {
		// else we have a valid value and can return
		return value.Value
	}
}

func (l loadingCache[K, V]) Put(key K, value V) {
	keyLock := l.getKeyLockFromMap(key)
	keyLock.Lock()
	defer keyLock.Unlock()

	// put new data in map
	l.dataMap[key] = &CacheValue[V]{
		Value:      value,
		Expiration: time.Now().Add(l.cacheDuration),
	}
}

/**
Claims the lockMapLock which is responsible for making sure there are no race conditions in creating keyLocks
tries to fetch lock from map, if none exists creates, puts in map and returns
*/
func (this *loadingCache[K, V]) getKeyLockFromMap(key K) *sync.Mutex {
	this.lockMapLock.Lock()
	defer this.lockMapLock.Unlock()

	if lock, exists := this.lockMap[key]; exists {
		return lock
	} else {
		newLock := &sync.Mutex{}
		this.lockMap[key] = newLock
		return newLock
	}
}
