import { execFileSync, spawn } from 'node:child_process'
import { createGzip } from 'node:zlib'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

// Сборка образов — минуты и сеть, поэтому отдельно от check:messenger. Проверки
// образа мессенджера — размер, пользователь, version (scripts/check-image.sh) и smoke
// (scripts/smoke.sh) — те же скрипты, что запускает джоба image в CI; trivy там —
// действие trivy-action той же версии.
const MESSENGER = fileURLToPath(new URL('../../messenger', import.meta.url))
const EXAMPLES = fileURLToPath(new URL('../modules/01-go-service/examples/dockerfiles', import.meta.url))
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

// Срок на docker save одного варианта: naive — сотни мегабайт, это десятки секунд.
// Без срока зависший save повесил бы check:images навсегда.
const SAVE_TIMEOUT_MS = 300_000

// Сжатый размер образа: байты `docker save` после gzip — тот же способ, что в
// scripts/check-image.sh, порог там же. Поток, а не буфер: архив варианта naive — сотни
// мегабайт.
function compressedSize(tag: string): Promise<number> {
  return new Promise((resolve, reject) => {
    const save = spawn('docker', ['save', tag], { cwd: MESSENGER, stdio: ['ignore', 'pipe', 'pipe'] })
    let stderr = ''
    let bytes = 0
    save.stderr.on('data', (chunk: Buffer) => { stderr += chunk.toString() })
    const gzip = createGzip()
    const timer = setTimeout(() => {
      save.kill('SIGKILL')
      gzip.destroy()
      reject(new Error(`docker save ${tag}: не завершился за ${SAVE_TIMEOUT_MS / 1000} с`))
    }, SAVE_TIMEOUT_MS)
    gzip.on('data', (chunk: Buffer) => { bytes += chunk.length })
    gzip.on('error', (err) => { clearTimeout(timer); reject(err) })
    save.stdout.pipe(gzip)
    let exitCode: number | null = null
    let gzipDone = false
    const finish = (): void => {
      if (exitCode === null || !gzipDone) return
      clearTimeout(timer)
      if (exitCode !== 0) reject(new Error(`docker save ${tag}: ${stderr || `код ${exitCode}`}`))
      else resolve(bytes)
    }
    gzip.on('end', () => { gzipDone = true; finish() })
    save.on('error', (err) => { clearTimeout(timer); reject(err) })
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
// Размер, пользователь и version — тем же скриптом, что в CI (порог — там).
console.log(run('bash', ['scripts/check-image.sh', 'messenger:local', 'check']).trim())
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
