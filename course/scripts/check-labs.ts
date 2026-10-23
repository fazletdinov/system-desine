import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const LABS = fileURLToPath(new URL('../../labs', import.meta.url))

function run(cmd: string, args: string[], timeout = 60_000, cwd = LABS): string {
  try {
    return execFileSync(cmd, args, {
      cwd,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
      timeout,
      maxBuffer: 64 * 1024 * 1024,
    })
  } catch (e) {
    const err = e as { stderr?: string; stdout?: string; message: string }
    console.error(`${cmd} ${args.join(' ')}:\n${err.stdout ?? ''}${err.stderr || err.message}`)
    process.exit(1)
  }
}

run('go', ['version'])
const unformatted = run('gofmt', ['-l', '.']).trim()
if (unformatted !== '') {
  console.error(`gofmt требует форматирования:\n${unformatted}`)
  process.exit(1)
}
run('go', ['vet', './...'])
run('go', ['test', '-race', './...'], 600_000)
run('docker', ['compose', '-f', '0b/compose.yaml', 'config', '-q'])
run('docker', ['compose', '-f', '1c/compose.yaml', 'config', '-q'])
// Сценарий 1c — настоящие namespaces и cgroup в privileged-контейнере: сборка образа
// при первом запуске и пять экспериментов.
const scenario = run('docker', ['compose', '-f', '1c/compose.yaml', 'run', '--rm', '--build', '-T', 'lab', '/lab/scenario.sh'], 900_000)
if (!scenario.includes('Все пять экспериментов прошли.')) {
  console.error(`Сценарий лабы 1c:\n${scenario}`)
  process.exit(1)
}
// Сценарий лабы 1.5 — маленькие Go-программы без Docker; намеренные гонки и утечки собираются с тегом labdemo.
const scenario15 = run('bash', ['1-5/scenario.sh'], 180_000)
if (scenario15.trim().split('\n').at(-1) !== 'Лаба 1.5: все проверки прошли.') {
  console.error(`Сценарий лабы 1.5:\n${scenario15}`)
  process.exit(1)
}
// Сценарий лабы 2a — PostgreSQL из 2a/compose.yaml (сценарий сам поднимает и убирает его),
// SQL-скрипты уроков 2.2–2.7 и программа keys на 10⁵ строк.
run('docker', ['compose', '-f', '2a/compose.yaml', 'config', '-q'])
const scenario2a = run('bash', ['2a/scenario.sh'], 300_000)
if (scenario2a.trim().split('\n').at(-1) !== 'Лаба 2a: все проверки прошли.') {
  console.error(`Сценарий лабы 2a:\n${scenario2a}`)
  process.exit(1)
}
// Сценарий лабы 0.4 в коротком режиме: память, fio, pg_test_fsync и ping — только порядок величины;
// results/ не трогает (полный прогон — bash 0-4/scenario.sh без флага).
run('docker', ['compose', '-f', '0-4/compose.yaml', 'config', '-q'])
const scenario04 = run('bash', ['0-4/scenario.sh', '--check'], 300_000)
if (scenario04.trim().split('\n').at(-1) !== 'Лаба 0.4: все проверки прошли.') {
  console.error(`Сценарий лабы 0.4:\n${scenario04}`)
  process.exit(1)
}
console.log('Лабораторные: gofmt, go vet, go test -race, compose config и сценарии 0.4, 1c, 1.5 и 2a — без ошибок.')
