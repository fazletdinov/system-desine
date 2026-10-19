package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// #region bench
// Цена самого ограничителя: задача почти пустая (атомарный инкремент), поэтому ns/op —
// накладные расходы на одну задачу при лимите concurrency.
func BenchmarkLimiterOverhead(b *testing.B) {
	var sink atomic.Int64
	tiny := func() { sink.Add(1) }

	b.Run("channel-semaphore", func(b *testing.B) {
		slots := make(chan struct{}, concurrency)
		var wg sync.WaitGroup
		for b.Loop() {
			slots <- struct{}{}
			wg.Go(func() {
				defer func() { <-slots }()
				tiny()
			})
		}
		wg.Wait()
	})
	b.Run("pool", func(b *testing.B) {
		jobs := make(chan func())
		var wg sync.WaitGroup
		for range concurrency {
			wg.Go(func() {
				for job := range jobs {
					job()
				}
			})
		}
		for b.Loop() {
			jobs <- tiny
		}
		close(jobs)
		wg.Wait()
	})
	b.Run("errgroup", func(b *testing.B) {
		var g errgroup.Group
		g.SetLimit(concurrency)
		for b.Loop() {
			g.Go(func() error { tiny(); return nil })
		}
		_ = g.Wait()
	})
	b.Run("weighted", func(b *testing.B) {
		ctx := context.Background()
		sem := semaphore.NewWeighted(concurrency)
		var wg sync.WaitGroup
		for b.Loop() {
			if err := sem.Acquire(ctx, 1); err != nil {
				b.Fatal(err)
			}
			wg.Go(func() {
				defer sem.Release(1)
				tiny()
			})
		}
		wg.Wait()
	})
}

// #endregion bench
