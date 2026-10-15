import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

// Сборка образов — минуты и сеть, поэтому отдельно от check:messenger. В CI то же
// самое делает джоба image (сборка, trivy, smoke).
const MESSENGER = fileURLToPath(new URL('../../messenger', import.meta.url))
const EXAMPLES = fileURLToPath(new URL('../modules/01-go-service/examples/dockerfiles', import.meta.url))
// Порог из спеки (§11): distroless static + статический бинарь мессенджера.
const MAX_IMAGE_BYTES = 25_000_000
const TRIVY = 'aquasec/trivy:0.74.0'

function run(cmd: string, args: string[], timeout = 900_000): string {
  try {
    return execFileSync(cmd, args, { cwd: MESSENGER, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], timeout, maxBuffer: 64 * 1024 * 1024 })
  } catch (e) {
    const err = e as { stderr?: string; stdout?: string; message: string }
    console.error(`${cmd} ${args.join(' ')}:\n${err.stdout ?? ''}${err.stderr || err.message}`)
    process.exit(1)
  }
}

function build(tag: string, dockerfile?: string): void {
  run('docker', ['build', '-q', '-t', tag, ...(dockerfile === undefined ? [] : ['-f', dockerfile]), '--build-arg', 'VERSION=check', '.'])
}

const inspect = (tag: string, format: string): string => run('docker', ['image', 'inspect', tag, '--format', format]).trim()

// Число уязвимостей HIGH и CRITICAL с исправлением; trivy читает образ через сокет Docker.
function vulns(tag: string): number {
  const json = run('docker', ['run', '--rm', '-v', '/var/run/docker.sock:/var/run/docker.sock', TRIVY, 'image', '--quiet', '--format', 'json', '--severity', 'HIGH,CRITICAL', '--ignore-unfixed', tag])
  const report = JSON.parse(json) as { Results?: { Vulnerabilities?: unknown[] }[] }
  return (report.Results ?? []).reduce((n, r) => n + (r.Vulnerabilities?.length ?? 0), 0)
}

build('messenger:local')
const size = Number(inspect('messenger:local', '{{.Size}}'))
const user = inspect('messenger:local', '{{.Config.User}}')
if (size >= MAX_IMAGE_BYTES || user !== '65532:65532') {
  console.error(`messenger:local: размер ${size} (порог ${MAX_IMAGE_BYTES}), пользователь «${user}» (ждали 65532:65532)`)
  process.exit(1)
}
if (run('docker', ['run', '--rm', 'messenger:local', 'version']).trim() !== 'check') {
  console.error('messenger:local version: ждали «check»')
  process.exit(1)
}
const mainVulns = vulns('messenger:local')
if (mainVulns > 0) {
  console.error(`messenger:local: уязвимостей HIGH/CRITICAL с исправлением — ${mainVulns}`)
  process.exit(1)
}

// Варианты для урока 1.16: собираются и сравниваются, но порогов не имеют.
const rows: [name: string, tag: string][] = [['distroless (наш)', 'messenger:local']]
for (const name of ['naive', 'alpine', 'scratch']) {
  const tag = `messenger-example:${name}`
  build(tag, join(EXAMPLES, `${name}.Dockerfile`))
  rows.push([name, tag])
}
for (const [name, tag] of rows) {
  const mb = (Number(inspect(tag, '{{.Size}}')) / 1e6).toFixed(1)
  console.log(`${name.padEnd(18)} ${mb.padStart(7)} МБ  HIGH/CRITICAL с исправлением: ${vulns(tag)}`)
}
console.log('Образы: сборка, размер, пользователь, version и trivy — без ошибок.')
