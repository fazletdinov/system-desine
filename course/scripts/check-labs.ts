import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const LABS = fileURLToPath(new URL('../../labs', import.meta.url))

function run(cmd: string, args: string[], cwd = LABS): string {
  try {
    return execFileSync(cmd, args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
  } catch (e) {
    const err = e as { stderr?: string; stdout?: string; message: string }
    console.error(`${cmd} ${args.join(' ')}:\n${err.stdout ?? ''}${err.stderr ?? err.message}`)
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
run('go', ['test', '-race', './...'])
run('docker', ['compose', '-f', '0b/compose.yaml', 'config', '-q'])
console.log('Лабораторные: gofmt, go vet, go test -race и compose config — без ошибок.')
