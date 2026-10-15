//go:build linux

// Command minicontainer — учебный контейнер на пару сотен строк: те же namespaces,
// pivot_root и cgroup v2, что использует runc, без всего остального (урок 1.15).
// Работает только от root и только внутри окружения лабы (docker compose).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "minicontainer:", errUsage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		code, err := run(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "minicontainer:", err)
		}
		os.Exit(code)
	case "child":
		// child возвращается, только если не смог стать командой пользователя.
		err := child(os.Args[2:])
		fmt.Fprintln(os.Stderr, "minicontainer child:", err)
		os.Exit(1)
	default:
		fmt.Fprintln(os.Stderr, "minicontainer:", errUsage)
		os.Exit(2)
	}
}

// #region clone
// run — родитель: создаёт cgroup, запускает себя же в новых namespaces и ждёт.
// Двухступенчатый запуск — как у runc: часть настройки (hostname, монтирование)
// можно сделать только изнутри новых namespaces, то есть в уже запущенном ребёнке.
func run(args []string) (int, error) {
	cfg, err := parseRun(args)
	if err != nil {
		return 2, err
	}
	if os.Geteuid() != 0 {
		return 1, errors.New("нужен root: запускайте внутри контейнера лабы (docker compose run lab)")
	}
	if err := checkControllers(cgroupRoot); err != nil {
		return 1, err
	}
	rootfs, err := filepath.Abs(cfg.Rootfs)
	if err != nil {
		return 1, fmt.Errorf("rootfs: %w", err)
	}
	cg, err := createCgroup(cgroupRoot, fmt.Sprintf("minicontainer-%d", os.Getpid()), cfg.Limits)
	if err != nil {
		return 1, err
	}
	defer func() {
		if err := cg.remove(); err != nil {
			fmt.Fprintln(os.Stderr, "minicontainer:", err)
		}
	}()
	dir, err := cg.open()
	if err != nil {
		return 1, err
	}
	// Дескриптор только для чтения: ошибка Close ничего не теряет.
	defer func() { _ = dir.Close() }()

	// Ctrl+C и docker stop не должны убить родителя раньше ребёнка: тогда отложенное
	// удаление группы не выполнится и она останется в /sys/fs/cgroup. Родитель ловит
	// сигнал сам и убивает ребёнка SIGKILL (Cancel по умолчанию у CommandContext):
	// ребёнок — init своего PID namespace, и SIGINT/SIGTERM без обработчика ядро ему
	// не доставит.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cmd := exec.CommandContext(ctx, "/proc/self/exe", append([]string{"child", cfg.Hostname, rootfs}, cfg.Argv...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Пять новых namespaces: имя хоста, номера процессов, точки монтирования,
		// сеть и System V IPC. User namespace не создаём — лаба работает от root.
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
		// clone3(CLONE_INTO_CGROUP): ребёнок под лимитами с первой инструкции.
		UseCgroupFD: true,
		CgroupFD:    int(dir.Fd()),
		// Родитель умер — ребёнок не должен пережить его и держать cgroup.
		Pdeathsig: syscall.SIGKILL,
	}
	runErr := cmd.Run()
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		return 1, fmt.Errorf("запуск: %w", runErr)
	}
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "minicontainer: получен сигнал остановки — контейнер убит SIGKILL")
	}
	report(cg)
	return exitCode(cmd.ProcessState), nil
}

// #endregion clone

// exitCode — как в shell: убитый сигналом N процесс даёт 128+N (OOM-kill → 137).
func exitCode(ps *os.ProcessState) int {
	ws, ok := ps.Sys().(syscall.WaitStatus)
	if ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}

// report печатает, сработали ли лимиты: это читает scenario.sh.
func report(cg *cgroup) {
	mem, merr := cg.events("memory.events")
	pids, perr := cg.events("pids.events")
	if err := errors.Join(merr, perr); err != nil {
		fmt.Fprintln(os.Stderr, "minicontainer: события cgroup:", err)
		return
	}
	fmt.Fprintf(os.Stderr, "minicontainer: итог: oom_kill=%d pids_max=%d\n", mem["oom_kill"], pids["max"])
}

// #region child
// child выполняется уже в новых namespaces: он PID 1 своего PID namespace.
func child(args []string) error {
	if len(args) < 3 {
		return errors.New("нужно: child <hostname> <rootfs> <команда> [аргументы…]")
	}
	// Вне нового PID namespace (если child вызвали руками) он сменил бы hostname и
	// корень самому окружению лабы.
	if os.Getpid() != 1 {
		return errors.New("child запускается только из run: ждали PID 1 в новом PID namespace")
	}
	host, rootfs, argv := args[0], args[1], args[2:]
	if err := syscall.Sethostname([]byte(host)); err != nil {
		return fmt.Errorf("sethostname: %w", err)
	}
	// Точки монтирования окружения обычно «shared»: без MS_PRIVATE наши mount и umount
	// распространились бы обратно в родительский mount namespace.
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount --make-rprivate /: %w", err)
	}
	// pivot_root требует, чтобы новый корень был точкой монтирования, — bind на себя.
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind %s: %w", rootfs, err)
	}
	oldroot := filepath.Join(rootfs, ".oldroot")
	if err := os.MkdirAll(oldroot, 0o700); err != nil {
		return fmt.Errorf("mkdir .oldroot: %w", err)
	}
	if err := syscall.PivotRoot(rootfs, oldroot); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}
	// Старый корень — вся файловая система окружения. Оставить его — оставить выход
	// из контейнера; в этом главное отличие pivot_root от chroot.
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("umount /.oldroot: %w", err)
	}
	if err := os.Remove("/.oldroot"); err != nil {
		return fmt.Errorf("rmdir /.oldroot: %w", err)
	}
	if err := mountProc(); err != nil {
		return err
	}
	if err := mountDev(); err != nil {
		return err
	}
	if err := os.Setenv("PATH", "/bin:/sbin:/usr/bin:/usr/sbin"); err != nil {
		return fmt.Errorf("PATH: %w", err)
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("команда %s: %w", argv[0], err)
	}
	// Exec заменяет child командой пользователя: она и становится PID 1.
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=/root", "TERM=" + os.Getenv("TERM")}
	return fmt.Errorf("exec %s: %w", path, syscall.Exec(path, argv, env))
}

// #endregion child

// mountProc монтирует свежий procfs: он показывает процессы *своего* PID namespace.
func mountProc() error {
	if err := os.MkdirAll("/proc", 0o555); err != nil {
		return fmt.Errorf("mkdir /proc: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	return nil
}

// devices — минимум, без которого не работают shell и большинство программ
// (фоновые задания shell читают /dev/null). runc создаёт тот же набор и ещё несколько.
var devices = []struct {
	name         string
	major, minor uint32
}{
	{"null", 1, 3},
	{"zero", 1, 5},
	{"urandom", 1, 9},
}

func mountDev() error {
	if err := os.MkdirAll("/dev", 0o755); err != nil {
		return fmt.Errorf("mkdir /dev: %w", err)
	}
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_NOEXEC, "mode=755,size=64k"); err != nil {
		return fmt.Errorf("mount /dev: %w", err)
	}
	for _, d := range devices {
		dev := int(d.major<<8 | d.minor) // makedev для старших/младших номеров < 256
		if err := syscall.Mknod("/dev/"+d.name, syscall.S_IFCHR|0o666, dev); err != nil {
			return fmt.Errorf("mknod /dev/%s: %w", d.name, err)
		}
	}
	return nil
}
