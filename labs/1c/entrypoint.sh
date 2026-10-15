#!/bin/sh
# region entrypoint
# В своём cgroup namespace контейнер лабы — корень иерархии, и в этом корне живут его
# процессы. Правило cgroup v2 «нет процессов во внутренних узлах»: включить контроллеры
# для дочерних групп нельзя, пока в группе есть процессы. Поэтому переносим все процессы
# в лист init и только потом включаем cpu, memory и pids — так же поступает entrypoint
# образа docker:dind.
set -eu

cg=/sys/fs/cgroup
if [ -w "$cg/cgroup.subtree_control" ] && ! grep -qw memory "$cg/cgroup.subtree_control"; then
	mkdir -p "$cg/init"
	# Процесс мог завершиться между чтением списка и записью — это не ошибка.
	while read -r pid; do
		echo "$pid" >"$cg/init/cgroup.procs" 2>/dev/null || true
	done <"$cg/cgroup.procs"
	echo "+cpu +memory +pids" >"$cg/cgroup.subtree_control"
fi
exec "$@"
# endregion entrypoint
