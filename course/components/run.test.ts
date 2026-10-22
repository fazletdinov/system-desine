import { describe, expect, it } from 'vitest'
import { inlineCode, runDir, runLine, worktreeAdd, worktreeRemove } from './run'

describe('runLine', () => {
  it('начинает команду с cd в каталог от корня репозитория', () => {
    expect(runLine('labs', 'go run ./1-5/sched/cost')).toBe('cd labs && go run ./1-5/sched/cost')
  })
  it('не трогает цепочку команд и кавычки', () => {
    expect(runLine('messenger', "scripts/overload.sh 50; docker inspect -f '{{.Id}}' x")).toBe(
      "cd messenger && scripts/overload.sh 50; docker inspect -f '{{.Id}}' x",
    )
  })
})

describe('рабочее дерево тега', () => {
  it('команда с тегом идёт из того же dir в дереве рядом с репозиторием', () => {
    expect(runLine('messenger', 'go run ./cmd/messenger', 'm01-l07')).toBe('cd ../sd-m01-l07/messenger && go run ./cmd/messenger')
  })
  it('без тега каталог — dir в самом репозитории', () => {
    expect(runDir('labs')).toBe('labs')
    expect(runDir('labs', 'm02-l01')).toBe('../sd-m02-l01/labs')
  })
  it('дерево создаётся и убирается одной командой из корня', () => {
    expect(worktreeAdd('m01-5-l10')).toBe('git worktree add ../sd-m01-5-l10 m01-5-l10')
    expect(worktreeRemove('m01-5-l10')).toBe('git worktree remove --force ../sd-m01-5-l10')
  })
})

describe('inlineCode', () => {
  it('делит текст на прозу и код в обратных кавычках', () => {
    expect(inlineCode('положите программу в `labs/x/main.go`, затем `go run ./x`')).toEqual([
      { code: false, text: 'положите программу в ' },
      { code: true, text: 'labs/x/main.go' },
      { code: false, text: ', затем ' },
      { code: true, text: 'go run ./x' },
    ])
  })
  it('строка из одного кода', () => {
    expect(inlineCode('`docker compose up -d`')).toEqual([{ code: true, text: 'docker compose up -d' }])
  })
  it('непарная кавычка остаётся текстом', () => {
    expect(inlineCode('текст `без конца')).toEqual([{ code: false, text: 'текст `без конца' }])
  })
  it('пустая строка — пустой список', () => {
    expect(inlineCode('')).toEqual([])
  })
})
