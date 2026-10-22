import { describe, expect, it } from 'vitest'
import { term } from './caption'
import { explanationTermIds, stepTermIds } from './explain'
import type { Step } from './types'

describe('stepTermIds', () => {
  it('собирает термины подписи, карточки и вопроса в порядке показа', () => {
    const step: Step<null> = {
      state: null,
      caption: [term('MVCC', 'mvcc')],
      card: { what: 'что', uses: 'чем', reads: 'как читать', why: [term('TID', 'tid')] },
      quiz: { question: 'вопрос', answer: [term('VACUUM', 'vacuum')] },
    }
    expect(stepTermIds(step)).toEqual(['mvcc', 'tid', 'vacuum'])
  })

  it('шаг без карточки и вопроса даёт только термины подписи', () => {
    const step: Step<null> = { state: null, caption: [term('MVCC', 'mvcc')] }
    expect(stepTermIds(step)).toEqual(['mvcc'])
  })
})

describe('explanationTermIds', () => {
  it('собирает термины из секций, here и real', () => {
    expect(
      explanationTermIds({
        title: 'Узел',
        kind: 'страница',
        sections: [
          { heading: 'а', body: [term('TID', 'tid')] },
          { heading: 'б', body: 'без терминов' },
        ],
        here: [term('MVCC', 'mvcc')],
        real: [term('VACUUM', 'vacuum')],
      }),
    ).toEqual(['tid', 'mvcc', 'vacuum'])
  })
})
