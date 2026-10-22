import { describe, expect, it } from 'vitest'
import { inlineCode, runLine } from './run'

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
