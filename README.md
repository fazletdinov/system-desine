# System Design на практике

Личный курс по проектированию высоконагруженных систем на Go: сайт курса с пошаговыми визуализациями (`course/`) и сквозной проект-мессенджер (`messenger/`, появится в модуле 1).

## Запуск курса

```bash
cd course
npm install
npm run dev        # http://localhost:5173
```

## Проверки

```bash
npm test           # Vitest: логика плеера, сценарии, расчёты, глоссарий, сайдбар
npm run typecheck  # vue-tsc
npm run build      # сборка; падает на неизвестном термине <T id="...">
```

## Как добавить урок

1. Файл `course/modules/NN-slug/NN-lesson.md` с frontmatter `title:` — он сам появится в меню.
2. Термины: `<T id="...">слово</T>`, определение — в `course/glossary/terms.yaml`.
3. Визуализация: сценарий в `course/viz/simulations/` (чистая функция + тест), рендерер в `course/viz/renderers/`, сцена в `course/viz/scenes/`; цвета — из `course/viz/palette.css` (см. `course/viz/README.md`).
4. Перед коммитом — чек-листы подробности и визуализации из спецификации (разделы 2.2 и 2.3).

Спецификация: `docs/superpowers/specs/2026-10-09-course-platform-design.md`.
