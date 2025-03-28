/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package helper rules cache
package subscribefilter

import (
	"sync"

	"chainmaker.org/chainmaker/pb-go/v2/txassign"
)

var (
	// once
	once sync.Once
	// _instance
	_instance *ruleCacheFactory
)

// ruleCacheFactory
type ruleCacheFactory struct {
	cache *ruleCache
}

// Factory fn
func Factory() *ruleCacheFactory {
	once.Do(func() { _instance = &ruleCacheFactory{cache: &ruleCache{m: sync.Map{}, l: sync.Mutex{}}} })
	return _instance
}

// ruleCache rule cache key: string, value: *txassign.FilterRule
type ruleCache struct {
	// cache height
	height uint64
	// m thread safe map
	m sync.Map
	// l locker
	l sync.Mutex
}

// Put value
func (r *ruleCache) Put(height uint64, key string, value *txassign.FilterRule) {
	r.m.Store(key, value)
	r.height = height
}

// Get value
func (r *ruleCache) Get(key string) (*txassign.FilterRule, uint64) {
	load, ok := r.m.Load(key)
	if !ok {
		return nil, 0
	}
	if load == nil {
		return nil, 0
	}
	return load.(*txassign.FilterRule), r.height
}
