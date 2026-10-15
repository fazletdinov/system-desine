import { describe, expect, it } from 'vitest'
import { parseBank } from './schema'

const question = {
  id: 'ports-at-consumer',
  lesson: '01-go-service/04-usecases',
  topic: 'Архитектура',
  level: 'M+',
  question: 'Почему порт объявляют в пакете потребителя?',
  answer: 'Потому что…',
  checklist: ['правило зависимостей', 'узкий интерфейс', 'подмена в тестах'],
  red_flags: ['«так принято»'],
  follow_ups: [{ q: 'А если потребителей два?', a: 'У каждого свой узкий интерфейс…' }],
}
const bank = { module: '01', title: 'Модуль 1', questions: [question] }

describe('parseBank', () => {
  it('принимает корректный банк', () => {
    expect(parseBank(bank, 't.yaml').questions[0]?.id).toBe('ports-at-consumer')
  })
  it.each([
    ['нет вопросов', { ...bank, questions: [] }],
    ['уровень не M+/S', { ...bank, questions: [{ ...question, level: 'Junior' }] }],
    ['чеклист короче 3', { ...bank, questions: [{ ...question, checklist: ['a', 'b'] }] }],
    ['чеклист длиннее 6', { ...bank, questions: [{ ...question, checklist: ['1', '2', '3', '4', '5', '6', '7'] }] }],
    ['нет красных флагов', { ...bank, questions: [{ ...question, red_flags: [] }] }],
    ['фоллоу-апов больше 3', { ...bank, questions: [{ ...question, follow_ups: [question.follow_ups[0], question.follow_ups[0], question.follow_ups[0], question.follow_ups[0]] }] }],
    ['пустой ответ', { ...bank, questions: [{ ...question, answer: ' ' }] }],
    ['повтор id', { ...bank, questions: [question, question] }],
    ['id не kebab-case', { ...bank, questions: [{ ...question, id: 'Порты' }] }],
    ['урок без модуля', { ...bank, questions: [{ ...question, lesson: '04-usecases' }] }],
    ['не объект', 'строка'],
  ])('отклоняет: %s', (_, raw) => {
    expect(() => parseBank(raw, 't.yaml')).toThrow(/t\.yaml/)
  })
})
