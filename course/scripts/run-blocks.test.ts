import { describe, expect, it } from 'vitest'
import { applyIllustrative, commandBlocksWithoutRun, type ComposeMounts, machinesWithoutRun, type RunTag, runRefs, scanRunBlocks } from './run-blocks'

const noMounts: ComposeMounts = { mounts: () => new Map() }
const lab2aMounts: ComposeMounts = {
  mounts: (file) => (file === 'labs/2a/compose.yaml' ? new Map([['/lab', 'labs/2a']]) : new Map()),
}
const run = (cmd: string, extra: Partial<RunTag> = {}): RunTag => ({ line: 1, dir: 'labs', cmd, ...extra })
const paths = (r: RunTag, mounts = noMounts) => runRefs(r, mounts).map((ref) => `${ref.kind} ${ref.path}`)

describe('scanRunBlocks', () => {
  it('находит абзацы «машина» и блоки <Run> с номерами строк', () => {
    const md = '```text\n$ go run ./x\n```\n\n<Run dir="labs" cmd="go run ./x" />\n\nмашина: i5.\n'
    const scan = scanRunBlocks(md)
    expect(scan.machines).toEqual([7])
    expect(scan.runs).toEqual([{ line: 5, dir: 'labs', cmd: 'go run ./x' }])
    expect(scan.problems).toEqual([])
  })
  it('раскодирует сущности в атрибутах, как Vue', () => {
    const md = '<Run dir="labs" cmd="a &amp;&amp; echo &quot;x&quot;" needs="docker" before="`up`" />'
    expect(scanRunBlocks(md).runs[0]).toMatchObject({ cmd: 'a && echo "x"', needs: 'docker', before: '`up`' })
  })
  it('не видит «машина» и <Run> внутри блока кода', () => {
    const scan = scanRunBlocks('```text\nмашина: внутри\n<Run dir="labs" cmd="x" />\n```')
    expect(scan.machines).toEqual([])
    expect(scan.runs).toEqual([])
  })
  it.each([
    ['<Run dir="labs" />', 'нет cmd'],
    ['<Run cmd="x" />', 'нет dir'],
    ['<Run dir="labs" cmd="x" needs="go" />', 'needs="go": допустимо только docker'],
    ['<Run dir="labs" cmd="x" size="1" />', 'неизвестный атрибут size'],
    ['<Run dir="../x" cmd="x" />', 'dir «../x» должен идти от корня репозитория'],
  ])('ошибка в теге %s', (tag, message) => {
    expect(scanRunBlocks(tag).problems).toEqual([{ line: 1, message }])
  })
  it('незакрытый тег — ошибка разбора, а не пропуск', () => {
    expect(scanRunBlocks('<Run dir="labs" cmd="x">').problems[0]?.message).toMatch(/не разобрать/)
  })
})

describe('machinesWithoutRun', () => {
  it('каждому абзацу «машина» нужен свой <Run> после предыдущего абзаца', () => {
    const md = ['<Run dir="labs" cmd="a" />', 'машина: 1', 'текст', 'машина та же', '<Run dir="labs" cmd="b" />', 'машина та же'].join('\n')
    expect(machinesWithoutRun(scanRunBlocks(md))).toEqual([4])
  })
  it('<Run> после абзаца не засчитывается предыдущему прогону', () => {
    expect(machinesWithoutRun(scanRunBlocks('машина: 1\n<Run dir="labs" cmd="a" />'))).toEqual([1])
  })
})

describe('commandBlocksWithoutRun', () => {
  const block = (cmd: string) => ['```text', `$ ${cmd}`, 'вывод', '```']
  const uncovered = (lines: string[]) => commandBlocksWithoutRun(scanRunBlocks(lines.join('\n'))).map((b) => `${b.line} ${b.command}`)

  it('блок «$ …» без <Run> в разделе — непокрыт, с номером строки ограды', () => {
    expect(uncovered(['## Раздел', '', ...block('go test ./...')])).toEqual(['3 go test ./...'])
  })
  it('сессия из нескольких блоков в разделе делит один <Run>', () => {
    const md = ['## Сессия', ...block('curl a'), '<Run dir="messenger" cmd="curl a" />', ...block('curl b'), ...block('curl c')]
    expect(uncovered(md)).toEqual([])
  })
  it('<Run> подраздела не покрывает блок раздела выше', () => {
    const md = ['## Первый', ...block('a'), '### Деталь', '<Run dir="labs" cmd="a" />']
    expect(uncovered(md)).toEqual(['2 a'])
  })
  it('<Run> над блоком не покрывает его: он о чём-то другом', () => {
    const md = ['## Раздел', '<Run dir="labs" cmd="x" />', ...block('a')]
    expect(uncovered(md)).toEqual(['3 a'])
  })
  it('блок с отступом в пункте списка — тоже команда', () => {
    const md = ['## Раздел', '1. Шаг:', '', '   ```text', '   $ gh attestation verify x', '   ```']
    expect(uncovered(md)).toEqual(['4 gh attestation verify x'])
  })
  it('<Run> соседнего подраздела того же уровня не засчитывается', () => {
    const md = ['## Раздел', '### А', ...block('a'), '### Б', '<Run dir="labs" cmd="b" />']
    expect(uncovered(md)).toEqual(['3 a'])
  })
  it('блок без «$ » в первой строке и «$ » не в первой строке — не команда', () => {
    expect(uncovered(['```text', 'вывод', '$ поздно', '```', '```go', 'package main', '```'])).toEqual([])
  })
  it('«# комментарий» внутри блока — не заголовок раздела', () => {
    const md = ['## Раздел', ...block('a'), '```text', '# не заголовок', '```', '<Run dir="labs" cmd="a" />']
    expect(uncovered(md)).toEqual([])
  })
})

describe('applyIllustrative', () => {
  const at = (file: string, line: number, command: string) => ({ file, block: { line, command } })
  const entry = { file: 'a.md', command: 'gh x', reason: 'нужен GHCR' }
  it('блок из списка — не ошибка, остальные — ошибки', () => {
    const r = applyIllustrative([at('a.md', 3, 'gh x'), at('a.md', 9, 'go test')], [entry])
    expect(r.problems).toEqual([at('a.md', 9, 'go test')])
    expect(r.stale).toEqual([])
  })
  it('запись, которой не соответствует ни один блок, — устаревшая', () => {
    expect(applyIllustrative([], [entry]).stale).toEqual([entry])
  })
  it('совпадение только по файлу и команде вместе', () => {
    expect(applyIllustrative([at('b.md', 3, 'gh x')], [entry]).problems).toHaveLength(1)
  })
})

describe('проверка тега <Run>', () => {
  it.each([
    ['<Run dir="messenger" tag="m01-l07" cmd="curl x" />', null],
    ['<Run dir="messenger" tag="m01 l07" cmd="curl x" />', 'tag="m01 l07": это не имя тега'],
    ['<Run dir="labs" cmd="cd 1-5/alloc &amp;&amp; go build ./escape" />', 'cd в cmd: каталог задаётся в dir'],
  ])('%s', (tag, message) => {
    const scan = scanRunBlocks(tag)
    expect(scan.problems[0]?.message ?? null).toBe(message)
  })
  it('тег попадает в разобранный <Run>', () => {
    expect(scanRunBlocks('<Run dir="messenger" tag="m01-l07" cmd="curl x" />').runs[0]?.tag).toBe('m01-l07')
  })
  it('cd внутри sh -c контейнера — не смена каталога команды', () => {
    expect(scanRunBlocks(`<Run dir="labs/1c" cmd="docker compose run --rm lab sh -c 'cd /mnt &amp;&amp; ls'" />`).problems).toEqual([])
  })
})

describe('runRefs', () => {
  it('каталог запуска проверяется всегда', () => {
    expect(paths(run('echo'))).toEqual(['dir labs'])
  })
  it('go run: пакет от каталога, аргументы программы — мимо', () => {
    expect(paths(run('go run ./2a/keys -n 1000000 ./not-a-package'))).toEqual(['dir labs', 'package labs/2a/keys'])
  })
  it('go test с шаблоном ./... проверяет каталог', () => {
    expect(paths(run('go test -race ./internal/...', { dir: 'messenger' }))).toEqual(['dir messenger', 'dir messenger/internal'])
  })
  it('пакет «.» — сам каталог', () => {
    expect(paths(run('go build -gcflags=-m=1 . 2>&1 | grep return.go', { dir: 'labs/x' }))).toEqual(['dir labs/x', 'package labs/x'])
  })
  it('бинарник из go build -o не требует файла в репозитории', () => {
    expect(paths(run('go build -o gctrace ./1-5/gc/gctrace && ./gctrace -live 64MiB'))).toEqual(['dir labs', 'package labs/1-5/gc/gctrace'])
  })
  it('бинарник, собранный в before, тоже', () => {
    expect(paths(run('./netpoll serve -n 3', { before: '`go build -o netpoll ./1-5/netpoll`; потом клиент' }))).toEqual([
      'dir labs',
      'package labs/1-5/netpoll',
    ])
  })
  it('compose -f и psql -f /lab/… внутри sh -c — через точки монтирования', () => {
    const cmd = "docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/anomalies.sql 2>&1'"
    expect(paths(run(cmd), lab2aMounts)).toEqual(['dir labs', 'file labs/2a/compose.yaml', 'file labs/2a/anomalies.sql'])
  })
  it('файл в контейнере без известного монтирования не проверяется', () => {
    expect(paths(run("docker compose exec db sh -c 'psql -f /tmp/x.sql'", { dir: 'messenger' }))).toEqual(['dir messenger'])
  })
  it('скрипты и ./файлы проверяются, перенаправление вывода — нет', () => {
    expect(paths(run('scripts/overload.sh 50 > out.txt; ./run.sh', { dir: 'messenger' }))).toEqual([
      'dir messenger',
      'file messenger/scripts/overload.sh',
      'file messenger/run.sh',
    ])
  })
  it('слова с подстановками оболочки пропускаются', () => {
    expect(paths(run('cat ./$(id)/x "$PWD/y" ./z*', { dir: 'messenger' }))).toEqual(['dir messenger'])
  })
  it('программа, которую before велит положить, не требует пакета; путь — от dir', () => {
    const r = run('go run ./threads', { before: 'положите программу выше в `threads/main.go`' })
    expect(paths(r)).toEqual(['dir labs'])
  })
  it('каталог запуска, который создаёт before, не проверяется', () => {
    const r = run('go build -o demo . && ./demo', { dir: 'labs/demo', before: 'создайте каталог и положите программу в `main.go`' })
    expect(paths(r)).toEqual([])
  })
  it('путь в before без «положите … в» должен существовать', () => {
    const r = run('go run ./nonexistent', { before: 'смотрите `x/README.md`' })
    expect(paths(r)).toEqual(['dir labs', 'file labs/x/README.md', 'package labs/nonexistent'])
  })
  it('git show ревизия:путь — путь от корня в этой ревизии', () => {
    const r = run('go tool pprof cpu.pprof', { dir: 'messenger', before: 'рецепт — `git show m02-l01:labs/1-5/results/profile/profile-load.txt`' })
    expect(runRefs(r, noMounts).map((ref) => `${ref.rev ?? '-'} ${ref.path}`)).toEqual(['- messenger', 'm02-l01 labs/1-5/results/profile/profile-load.txt'])
  })
  it('файлы из before проверяются от каталога запуска', () => {
    const r = run('go tool pprof cpu.pprof', { dir: 'messenger', before: 'как в `../labs/1-5/results/profile/profile-load.txt`' })
    expect(paths(r)).toEqual(['dir messenger', 'file labs/1-5/results/profile/profile-load.txt'])
  })
  it('цель cp и файлы профилей из флагов go test — порождены командой', () => {
    const cmd = "go test -bench=Route -cpuprofile=cpu.pprof ./1-5/pgo/ && cp ./cpu.pprof ./1-5/pgo/default.pgo && cat ./cpu.pprof"
    expect(paths(run(cmd))).toEqual(['dir labs', 'package labs/1-5/pgo'])
  })
  it('go generate проверяет шаблон пакетов как каталог', () => {
    expect(paths(run('go generate ./internal/...', { dir: 'messenger' }))).toEqual(['dir messenger', 'dir messenger/internal'])
  })
  it('docker build -f: путь к Dockerfile проверяется от каталога', () => {
    expect(paths(run('docker build -t x -f ../course/a.Dockerfile .', { dir: 'messenger' }))).toEqual(['dir messenger', 'file course/a.Dockerfile'])
  })
})
