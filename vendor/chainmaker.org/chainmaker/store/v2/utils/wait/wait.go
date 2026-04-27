/*
 * Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package wait

import "sync"

// Group allows join a set of goroutines and wait for them to finish
type Group struct {
	wg sync.WaitGroup
}

// Wait waits for all goroutines to finish
func (g *Group) Wait() {
	g.wg.Wait()
}

// JoinWithChannel starts f in a new goroutine and join it to group.
// stopCh is passed to f as an argument. f should stop when stopCh is available.
func (g *Group) JoinWithChannel(stopCh <-chan struct{}, fn func(stopCh <-chan struct{})) {
	g.Join(func() {
		fn(stopCh)
	})
}

// Join starts f in a new goroutine and join it to group.
func (g *Group) Join(fn func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		fn()
	}()
}
