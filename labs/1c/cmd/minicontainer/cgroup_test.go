//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckControllers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cgroup.subtree_control"), "cpu memory pids\n")
	if err := checkControllers(root); err != nil {
		t.Fatalf("все контроллеры включены: %v", err)
	}
	writeFile(t, filepath.Join(root, "cgroup.subtree_control"), "cpu io\n")
	err := checkControllers(root)
	if err == nil || !strings.Contains(err.Error(), "memory") {
		t.Fatalf("без memory: ошибка %v, ждали упоминание memory", err)
	}
	if err := checkControllers(filepath.Join(root, "нет")); err == nil {
		t.Fatal("нет cgroup v2: ждали ошибку")
	}
}

func TestCreateCgroupWritesLimits(t *testing.T) {
	root := t.TempDir()
	l := limits{MemoryMax: "33554432", PidsMax: "16", CPUMax: "25000 100000"}
	cg, err := createCgroup(root, "minicontainer-1", l)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cg.path) })
	for file, want := range map[string]string{"memory.max": l.MemoryMax, "memory.swap.max": "0", "pids.max": l.PidsMax, "cpu.max": l.CPUMax} {
		b, err := os.ReadFile(filepath.Join(cg.path, file))
		if err != nil || string(b) != want {
			t.Errorf("%s = %q, %v; ждали %q", file, b, err, want)
		}
	}
	if _, err := createCgroup(root, "minicontainer-1", l); err == nil {
		t.Error("каталог уже есть: ждали ошибку, а не чужие лимиты")
	}
}

func TestEvents(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "memory.events"), "low 0\nhigh 0\nmax 3\noom 1\noom_kill 1\noom_group_kill 0\n")
	cg := &cgroup{path: dir}
	ev, err := cg.events("memory.events")
	if err != nil || ev["oom_kill"] != 1 || ev["max"] != 3 {
		t.Fatalf("events = %v, %v", ev, err)
	}
	writeFile(t, filepath.Join(dir, "pids.events"), "max не-число\n")
	if _, err := cg.events("pids.events"); err == nil {
		t.Fatal("мусор в файле: ждали ошибку")
	}
}
