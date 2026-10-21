import { execFileSync, spawn } from 'node:child_process'
import { createGzip } from 'node:zlib'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

// Сборка образов — минуты и сеть, поэтому отдельно от check:messenger. В CI то же
// самое делает джоба image (сборка, trivy, smoke).
const MESSENGER = fileURLToPath(new URL('../../messenger', import.meta.url))
const EXAMPLES = fileURLToPath(new URL('../modules/01-go-service/examples/dockerfiles', import.meta.url))
// Порог из спеки (§11): distroless static + статический бинарь мессенджера. Меряем
// сжатый размер — столько качается из реестра — и меряем сами, а не берём {{.Size}}:
// его смысл зависит от хранилища образов и даже от версии Docker:
// - overlay2 / graphdriver (возможен на раннере CI) — распакованные слои;
// - containerd image store на Docker 29.9.1 — место на диске: распакованные слои плюс
//   сжатые архивы (25,6 МБ у образа после 2a при сжатом 6,0 МБ), хотя в более ранних
//   версиях 29.x тот же запрос давал сжатый размер (3,84 МБ у образа урока 1.16).
// `docker save | gzip` даёт одно и то же на обоих хранилищах: overlay2 отдаёт в save
// распакованные слои, и gzip их сжимает; containerd отдаёт уже сжатые архивы слоёв, и
// повторный gzip почти ничего не меняет. Сумма size слоёв из манифеста была бы точнее,
// но на overlay2 сжатых слоёв нет, пока образ не отправлен в реестр.
// Замер (Docker 29.9.1, containerd): образ тега m01-l16 — 3 832 236 байт, после 2a
// (pgx и goose: бинарь 7,6 → 13,7 МБ) — 6 017 670 байт. 25 МБ — в четыре раза больше
// нынешнего, но регрессию вроде образа на полном golang (сотни МБ) порог ловит.
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

// Сжатый размер образа: байты `docker save` после gzip. Поток, а не буфер: архив
// варианта naive — сотни мегабайт.
function compressedSize(tag: string): Promise<number> {
  return new Promise((resolve, reject) => {
    const save = spawn('docker', ['save', tag], { cwd: MESSENGER, stdio: ['ignore', 'pipe', 'pipe'] })
    let stderr = ''
    let bytes = 0
    save.stderr.on('data', (chunk: Buffer) => { stderr += chunk.toString() })
    const gzip = createGzip()
    gzip.on('data', (chunk: Buffer) => { bytes += chunk.length })
    gzip.on('error', reject)
    save.stdout.pipe(gzip)
    let exitCode: number | null = null
    let gzipDone = false
    const finish = (): void => {
      if (exitCode === null || !gzipDone) return
      if (exitCode !== 0) reject(new Error(`docker save ${tag}: ${stderr || `код ${exitCode}`}`))
      else resolve(bytes)
    }
    gzip.on('end', () => { gzipDone = true; finish() })
    save.on('error', reject)
    save.on('close', (code) => { exitCode = code ?? 1; finish() })
  })
}

async function sizeOf(tag: string): Promise<number> {
  try {
    return await compressedSize(tag)
  } catch (e) {
    console.error(e instanceof Error ? e.message : String(e))
    process.exit(1)
  }
}

// Число уязвимостей HIGH и CRITICAL с исправлением; trivy читает образ через сокет Docker.
function vulns(tag: string): number {
  const json = run('docker', ['run', '--rm', '-v', '/var/run/docker.sock:/var/run/docker.sock', TRIVY, 'image', '--quiet', '--format', 'json', '--severity', 'HIGH,CRITICAL', '--ignore-unfixed', tag])
  const report = JSON.parse(json) as { Results?: { Vulnerabilities?: unknown[] }[] }
  return (report.Results ?? []).reduce((n, r) => n + (r.Vulnerabilities?.length ?? 0), 0)
}

build('messenger:local')
const size = await sizeOf('messenger:local')
const user = inspect('messenger:local', '{{.Config.User}}')
if (size >= MAX_IMAGE_BYTES || user !== '65532:65532') {
  console.error(`messenger:local: сжатый размер ${size} (порог ${MAX_IMAGE_BYTES}), пользователь «${user}» (ждали 65532:65532)`)
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
// Smoke: контейнер становится healthy, отвечает на пробы и API, корректно
// останавливается по SIGTERM (тот же скрипт запускает CI).
const smoke = run('bash', ['scripts/smoke.sh', 'messenger:local'], 120_000).trim().split('\n')
console.log(smoke[smoke.length - 1])

// Варианты для урока 1.16: собираются и сравниваются, но порогов не имеют.
const rows: [name: string, tag: string][] = [['distroless (наш)', 'messenger:local']]
for (const name of ['naive', 'alpine', 'scratch']) {
  const tag = `messenger-example:${name}`
  build(tag, join(EXAMPLES, `${name}.Dockerfile`))
  rows.push([name, tag])
}
for (const [name, tag] of rows) {
  const mb = ((await sizeOf(tag)) / 1e6).toFixed(1)
  console.log(`${name.padEnd(18)} ${mb.padStart(7)} МБ  HIGH/CRITICAL с исправлением: ${vulns(tag)}`)
}
console.log('Образы: сборка, размер, пользователь, version, trivy и smoke — без ошибок.')
