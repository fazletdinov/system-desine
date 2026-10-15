import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type StartPhase =
  | 'run'
  | 'clone'
  | 'child'
  | 'hostname'
  | 'private-mounts'
  | 'pivot-root'
  | 'umount-oldroot'
  | 'mount-proc'
  | 'exec'

export const START_PHASES: readonly StartPhase[] = [
  'run',
  'clone',
  'child',
  'hostname',
  'private-mounts',
  'pivot-root',
  'umount-oldroot',
  'mount-proc',
  'exec',
]

export interface ProcNode {
  pid: number
  nsPid?: number
  cmd: string
}

export interface ContainerStartFrame {
  phase: StartPhase
  host: ProcNode[]
  container: ProcNode[] | null
  mounts: { root: string; proc: boolean; oldroot: boolean }
  hostname: { host: string; container: string | null }
  // Флаги clone(2), которые подсвечиваются на фазе clone.
  flags: readonly string[]
}

const HOST_NAME = 'dev-laptop'
const CONTAINER_NAME = 'minicontainer'
const LAUNCHER_PID = 4200
const CHILD_PID = 4242
const CLONE_FLAGS: readonly string[] = ['CLONE_NEWUTS', 'CLONE_NEWPID', 'CLONE_NEWNS', 'CLONE_NEWNET', 'CLONE_NEWIPC']

const hostBase: ProcNode[] = [
  { pid: 1, cmd: 'systemd' },
  { pid: LAUNCHER_PID, cmd: 'minicontainer run' },
]
const withChild = (cmd: string): ProcNode[] => [...hostBase, { pid: CHILD_PID, nsPid: 1, cmd }]
const inside = (cmd: string): ProcNode[] => [{ pid: 1, cmd }]

const HOST_ROOT = '/ хоста'
const NEW_ROOT = '/ = rootfs'

interface Frame {
  state: ContainerStartFrame
  caption: Caption
}

function frame(
  phase: StartPhase,
  p: Pick<ContainerStartFrame, 'host' | 'container'> & { root: string; proc?: boolean; oldroot?: boolean; hostname?: string; flags?: readonly string[] },
  caption: Caption,
): Frame {
  return {
    state: {
      phase,
      host: p.host,
      container: p.container,
      mounts: { root: p.root, proc: p.proc ?? false, oldroot: p.oldroot ?? false },
      hostname: { host: HOST_NAME, container: p.hostname ?? null },
      flags: p.flags ?? [],
    },
    caption,
  }
}

const FRAMES: readonly Frame[] = [
  frame('run', { host: hostBase, container: null, root: HOST_ROOT }, [
    { text: 'Пользователь запускает ' },
    { text: 'minicontainer run' },
    { text: ': обычный процесс на хосте. Никакого «контейнера» ещё нет, есть только набор системных вызовов, которые сейчас будут выполнены по порядку. Контейнер — это обычный процесс, которому ядро показывает урезанный мир.' },
  ]),
  frame('clone', { host: hostBase, container: null, root: HOST_ROOT, flags: CLONE_FLAGS }, [
    { text: 'Запускающий процесс вызывает clone(2) с пятью флагами: UTS, PID, mount, net и IPC. Каждый флаг создаёт новое ' },
    term('пространство имён (namespace)', 'namespace'),
    { text: ' и сразу помещает в него нового ребёнка. Пространство имён ничего не ограничивает по количеству, оно только прячет чужое.' },
  ]),
  frame('child', { host: withChild('/proc/self/exe child'), container: inside('/proc/self/exe child'), root: HOST_ROOT }, [
    { text: 'Ребёнок — копия того же исполняемого файла с аргументом child. У него два номера: на хосте PID 4242, внутри нового PID-namespace — ' },
    term('PID 1', 'pid-1'),
    { text: '. Один процесс, два имени: хост видит его в обычном ps, сам он считает себя первым и единственным.' },
  ]),
  frame('hostname', { host: withChild('/proc/self/exe child'), container: inside('/proc/self/exe child'), root: HOST_ROOT, hostname: CONTAINER_NAME }, 'Внутри нового UTS-namespace ребёнок меняет имя хоста на minicontainer. Имя хоста на самом хосте осталось прежним: вызов sethostname затрагивает только свой UTS-namespace. Это самая простая демонстрация того, что изоляция настоящая.'),
  frame('private-mounts', { host: withChild('/proc/self/exe child'), container: inside('/proc/self/exe child'), root: '/ хоста (propagation: private)', hostname: CONTAINER_NAME }, 'Все точки монтирования помечаются private. Без этого следующие шаги «протекли» бы наружу: по умолчанию события монтирования распространяются между копиями mount-namespace, и смена корня в контейнере сломала бы хост.'),
  frame('pivot-root', { host: withChild('/proc/self/exe child'), container: inside('/proc/self/exe child'), root: `${NEW_ROOT}, старый корень в /.oldroot`, oldroot: true, hostname: CONTAINER_NAME }, [
    { text: 'Вызов ' },
    term('pivot-root', 'pivot-root'),
    { text: ' делает корнем каталог с распакованным образом — ' },
    term('rootfs', 'rootfs'),
    { text: ', а прежний корень хоста переезжает в /.oldroot. Пока он там лежит, из контейнера ещё можно дотянуться до файлов хоста.' },
  ]),
  frame('umount-oldroot', { host: withChild('/proc/self/exe child'), container: inside('/proc/self/exe child'), root: NEW_ROOT, hostname: CONTAINER_NAME }, 'Старый корень отцепляется (umount) и каталог /.oldroot удаляется. С этого момента путь к хостовой файловой системе из контейнера не найти: ни по имени, ни через «..». Это и есть граница, ради которой нужен pivot_root, а не chroot.'),
  frame('mount-proc', { host: withChild('/proc/self/exe child'), container: inside('/proc/self/exe child'), root: `${NEW_ROOT}, /proc смонтирован`, proc: true, hostname: CONTAINER_NAME }, 'Внутри нового mount-namespace монтируется свой /proc. Он строится по PID-namespace контейнера, поэтому ps и top внутри видят только процессы контейнера. Смонтируй мы /proc до смены корня, там остался бы список процессов хоста.'),
  frame('exec', { host: withChild('sh'), container: inside('sh'), root: `${NEW_ROOT}, /proc смонтирован`, proc: true, hostname: CONTAINER_NAME }, [
    { text: 'Последний вызов — exec: процесс заменяет себя пользовательской командой, здесь sh. PID не меняется: на хосте всё тот же 4242, теперь с командой sh, внутри — ' },
    term('PID 1', 'pid-1'),
    { text: '. Контейнер готов; он живёт, пока жив этот процесс, и сигналы теперь адресуются именно ему.' },
  ]),
]

export function containerStartScenario(): readonly Step<ContainerStartFrame>[] {
  return FRAMES.map(({ state, caption }) => ({ state, caption }))
}
