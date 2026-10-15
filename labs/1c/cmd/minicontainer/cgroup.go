//go:build linux

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const cgroupRoot = "/sys/fs/cgroup"

// requiredControllers — без них лимиты из флагов некуда записать: файлы memory.max,
// pids.max и cpu.max появляются в дочерней группе, только если контроллер включён
// в cgroup.subtree_control родителя.
var requiredControllers = []string{"cpu", "memory", "pids"}

type cgroup struct{ path string }

func checkControllers(root string) error {
	b, err := os.ReadFile(filepath.Join(root, "cgroup.subtree_control"))
	if err != nil {
		return fmt.Errorf("нет cgroup v2 в %s: %w", root, err)
	}
	enabled := strings.Fields(string(b))
	for _, c := range requiredControllers {
		if !slices.Contains(enabled, c) {
			return fmt.Errorf("контроллер %s не включён в %s/cgroup.subtree_control — запускайте лабу через docker compose: entrypoint.sh включает контроллеры", c, root)
		}
	}
	return nil
}

// #region cgroup-limits
// createCgroup создаёт группу и пишет лимиты до того, как в неё попадёт процесс:
// иначе первые миллисекунды он работал бы без ограничений.
func createCgroup(root, name string, l limits) (*cgroup, error) {
	path := filepath.Join(root, name)
	// Mkdir, а не MkdirAll: существующая группа — чужая или оставшаяся от упавшего
	// запуска, и молча писать в неё свои лимиты нельзя.
	if err := os.Mkdir(path, 0o755); err != nil {
		return nil, fmt.Errorf("создать cgroup: %w", err)
	}
	cg := &cgroup{path: path}
	settings := []struct{ file, value string }{
		{"memory.max", l.MemoryMax},
		// Без swap лимит памяти — жёсткий: процесс получит OOM-kill, а не уйдёт в своп.
		{"memory.swap.max", "0"},
		{"pids.max", l.PidsMax},
		{"cpu.max", l.CPUMax},
	}
	for _, s := range settings {
		err := os.WriteFile(filepath.Join(path, s.file), []byte(s.value), 0o644)
		// memory.swap.max нет, если ядро собрано без учёта swap: тогда и swap не грозит.
		if errors.Is(err, fs.ErrNotExist) && s.file == "memory.swap.max" {
			continue
		}
		if err != nil {
			return nil, errors.Join(fmt.Errorf("%s: %w", s.file, err), cg.remove())
		}
	}
	return cg, nil
}

// #endregion cgroup-limits

// open даёт дескриптор каталога группы для clone3(CLONE_INTO_CGROUP): ребёнок
// рождается сразу внутри группы.
func (c *cgroup) open() (*os.File, error) {
	f, err := os.Open(c.path)
	if err != nil {
		return nil, fmt.Errorf("открыть cgroup: %w", err)
	}
	return f, nil
}

// events читает файл вида «ключ значение» (memory.events, pids.events, cpu.stat).
func (c *cgroup) events(file string) (map[string]int64, error) {
	b, err := os.ReadFile(filepath.Join(c.path, file))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	out := map[string]int64{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, v, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("%s: строка %q", file, line)
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q: %w", file, line, err)
		}
		out[k] = n
	}
	return out, nil
}

// removeAttempts × removeBackoff — сколько ждём, пока ядро отпустит группу: после
// выхода процесса она бывает «занята» ещё доли миллисекунды.
const (
	removeAttempts = 20
	removeBackoff  = 10 * time.Millisecond
)

// remove удаляет группу. В cgroupfs rmdir удаляет каталог вместе с файлами
// интерфейса, если в группе нет процессов.
func (c *cgroup) remove() error {
	var err error
	for range removeAttempts {
		if err = os.Remove(c.path); err == nil || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		time.Sleep(removeBackoff)
	}
	return fmt.Errorf("удалить cgroup %s: %w", c.path, err)
}
