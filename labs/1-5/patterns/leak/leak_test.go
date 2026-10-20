//go:build labdemo

package main

import (
	"testing"
	"testing/synctest"
)

// #region leak-test
// Утечку ловит сам synctest.Test: когда функция теста вернулась, а в пузыре осталась
// заблокированная горутина, Test паникует и печатает её стек. Время пузыря после возврата
// не идёт, поэтому горутина в time.Sleep тоже «заблокирована навсегда»: Test требует,
// чтобы ни одна горутина не пережила проверяемый вызов. Первые два теста падают
// намеренно — поэтому файл собирается только с тегом labdemo.
func TestProcessWorkItemsLeaks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if _, err := processWorkItems(items()); err == nil {
			t.Fatal("ждали ошибку задачи")
		}
	})
}

func TestProcessWorkItemsFixedOutlivesCall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if _, err := processWorkItemsFixed(items()); err == nil {
			t.Fatal("ждали ошибку задачи")
		}
	})
}

func TestProcessWorkItemsGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if _, err := processWorkItemsGroup(items()); err == nil {
			t.Fatal("ждали ошибку задачи")
		}
	})
}

// #endregion leak-test
