# Бэклог после модуля 0

Отложенные замечания ревью ветки feat/course-platform-module-0 (для подпроекта «Полировка модуля 0» и до старта модуля 0b).

## Запросы пользователя (полировка)
- Цвет со смыслом: фирменный акцент, цветные метки частей урока, Mermaid в палитре --viz-*, подсветка чисел; тёмная тема.
- Углубление теории: блоки «🔬 Под капотом», курс самодостаточен, источники — «читать дальше».
- Код уровня прода: фрагменты из реального кода мессенджера; упрощённые явно помечены.
- Полноэкранный режим для визуализаций (⛶, Esc; CSS-оверлей на iPhone).

## До модуля 0b
- Подписи шагов визуализации с терминами: формат сегментов {text, termId?} + тест на id из terms.yaml.

## Отложенные мелочи
- Task 1: minor (deferred): dev-chain npm audit 2 critical/1 high (not in prod build); esbuild postinstall skipped by npm allowScripts; tsconfig globs empty until later tasks
- Task 2: minor (deferred): YAML parse error in frontmatter lacks file path; module dirs not filtered by ^\d{2}-; temp dirs not cleaned in test; module without index.md yields dead link; sidebar computed once (dev restart needed for new lessons)
- Task 3: minor (deferred): terms-check regex misses single-quoted/bound ids; KNOWN_TERMS read once (dev restart); no test on real terms.yaml; ru collator puts Latin after Cyrillic; T.vue inlines whole glossary per page; GlossaryList key by link.title
- Task 3: minor (deferred): Esc-dismiss resets on mouseleave while focus inside; T_TAG case-sensitive, misses <T/>; role=tooltip containing a link
- Task 4: minor (deferred): stage-level arrows imply skipping through deepening nodes; elaboration ch.4 citation unverified; mermaid >500kB chunk warning; subgraph order comment; course map tiny at 375px (wrap Mermaid in scroll container)
- Task 5: minor (deferred): literal invisible NBSP chars in format.ts/test (use \u escapes); compact picks digits before rounding (999999ns→"1 000 мкс"); seek(NaN) → NaN index; format fns untested for NaN/negative
- Task 6: minor (deferred): lesson 0.3 heading still says "код работает 1 мс"; "Балансировщик" tight in narrow node
- Task 7: minor (deferred): viz/README legend lacks latency kind colours (cpu→service, memory→cache… inconsistent with "Кэш (Redis)"); times() 12–14 branch untested; Dean "с которого началась таблица" attribution (Norvig 2001 earlier); "≈474" exact
