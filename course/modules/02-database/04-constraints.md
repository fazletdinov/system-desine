---
title: "2.4. Ограничения"
---

<script setup>
import ConstraintRace from '../../viz/scenes/ConstraintRace.vue'
</script>

# 2.4. Ограничения

## <Part kind="problem" /> Проблема: правило, которое держит только код

В модуле 1 уникальность имени пользователя держало in-memory хранилище: «найти имя — если нет, записать» выполнялось под одним мьютексом, и две одновременные регистрации `alice` не могли проскочить вместе. Тест `storetest` проверял это так: 100 забегов, в каждом 16 горутин одновременно регистрируют одно имя, и выжить должен ровно один пользователь.

Теперь мессенджер хранит данные в PostgreSQL, и у этого мьютекса три проблемы. Он живёт в памяти **одного процесса**, а экземпляров сервиса за балансировщиком два или двадцать. Он не видит **второго писателя**: скрипт переноса данных, админку, соседний сервис. И он ничего не знает о **ручном SQL**, который дежурный выполнит ночью в `psql`. Правило «имя уникально» должно держаться при любом писателе, а единственное место, через которое проходят все писатели, — сама база.

Лаба `labs/2a/constraints.sql` нарушает каждое правило намеренно и показывает, как база отвечает. Вот вставка второго `alice` в таблицу с ограничением `UNIQUE`; `psql` с `\set VERBOSITY verbose` печатает все поля ошибки:

```text
INSERT INTO users (username) VALUES ('alice');
psql:/lab/constraints.sql:64: ERROR:  23505: duplicate key value violates unique constraint "users_username_key"
DETAIL:  Key (username)=(alice) already exists.
SCHEMA NAME:  constraints
TABLE NAME:  users
CONSTRAINT NAME:  users_username_key
LOCATION:  _bt_check_unique, nbtinsert.c:666
SQLSTATE 23505
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина: 12th Gen Intel Core i5-12400 (6 ядер, 12 логических процессоров), Ubuntu 24.04.5, ядро Linux 7.0, Docker 29.9.1, PostgreSQL 18.6 с настройками образа по умолчанию и данными на tmpfs, go1.27.2; полный вывод — `labs/2a/results/constraints.txt`.

В этом выводе всё, что нужно программе. Код `23505` говорит, **что** случилось: нарушена уникальность. Имя `users_username_key` говорит, **какое** правило нарушено. А `LOCATION` — функция `_bt_check_unique` в файле B-дерева — подсказывает, **где** живёт проверка: внутри уникального индекса. Это главный механизм урока, мы к нему вернёмся.

К концу урока вы:

- разделите инварианты мессенджера между доменом и базой и объясните, почему многие из них держат **оба**;
- выберете между `NOT NULL`, `CHECK`, `UNIQUE` (в том числе частичным и по выражению), `FOREIGN KEY` с нужным `ON DELETE` и `EXCLUDE`, и назовёте, чего каждое из них не умеет;
- объясните, когда нужны откладываемые ограничения, почему отложить можно только `UNIQUE`, `PRIMARY KEY`, `FOREIGN KEY` и `EXCLUDE`, и почему мессенджер не откладывает ничего;
- переведёте ошибку базы в доменную по коду SQLSTATE и имени ограничения, а не по тексту, и объясните, почему ошибка ограничения обрывает транзакцию и что из этого следует для `SendMessage`;
- прочитаете ADR-0014: схема на контекст и межсхемные внешние ключи к `users.users`.

## <Part kind="theory" /> Кто держит инвариант: домен, база или оба

### Ограничение — правило, которое проверяет база

<T id="invariant">Инвариант</T> (invariant) из урока 1.2 — условие, которое истинно всегда, пока объект существует: «имя пользователя от 3 до 32 символов», «у личного чата ровно два участника». В модуле 1 все инварианты охранял домен: конструкторы вроде `NewUsername` и методы агрегата вроде `Chat.Post`.

<T id="constraint">Ограничение целостности</T> (integrity constraint) — правило, которое база проверяет сама при **каждой** записи, кто бы её ни делал, и отвергает строку, которая его нарушает. В PostgreSQL их шесть видов: `NOT NULL`, `CHECK`, `UNIQUE`, `PRIMARY KEY`, `FOREIGN KEY` и `EXCLUDE`. Ограничение — часть схемы, оно объявляется в `CREATE TABLE` или `ALTER TABLE` и действует, пока его не удалят.

### Почему одного домена мало

У доменной проверки есть граница: она проверяет только то, что делает **этот** код в **этом** процессе. Три способа её обойти встречаются в любом живом сервисе.

1. **Гонка между процессами.** Два экземпляра одновременно регистрируют `alice`. Каждый делает `SELECT` по имени, каждый видит «свободно», каждый вставляет. Это <T id="check-then-act">гонка «проверь, потом действуй»</T> (check-then-act race): проверка верна в момент проверки, но между ней и записью есть окно, и чужая запись в него помещается. Мьютекс закрывал окно в модуле 1, потому что процесс был один. Детектор гонок Go её не найдёт: это не гонка данных в памяти, а гонка за порядок событий в базе.
2. **Второй писатель.** Скрипт миграции данных, задача в планировщике, сервис аналитики, который «только поправит статус», админ-панель на другом языке. Ни один из них не вызывает `NewUsername`.
3. **Ручной SQL.** `UPDATE users.users SET username = 'Alice Smith' WHERE …` из консоли дежурного проходит мимо всех проверок приложения.

Ограничение закрывает все три: базе неважно, откуда пришла строка.

Почему гонку из пункта 1 не закрывает сама база, раз оба `SELECT` идут в неё? Каждая транзакция PostgreSQL читает данные через <T id="snapshot">снимок</T> (snapshot) — набор версий строк, зафиксированных к определённому моменту; незафиксированных строк чужих транзакций в снимке нет никогда. Когда снимок берётся, задаёт <T id="isolation-level">уровень изоляции</T> (isolation level). По умолчанию в PostgreSQL — `READ COMMITTED`: свой снимок на каждую команду. `REPEATABLE READ` — один снимок на всю транзакцию. `SERIALIZABLE` — результат как при выполнении транзакций по очереди: заметив опасное пересечение, база откатывает одну из них с кодом `40001`, и её приходится повторять. На первых двух уровнях `SELECT` экземпляра B не видит строку, которую A вставил, но ещё не зафиксировал, и честно отвечает «свободно». Устройство снимков (<T id="mvcc">MVCC</T> — хранение нескольких версий строки; коротко — в углублении про `_bt_check_unique` ниже) и уровни изоляции подробно разбирает часть 2b; здесь достаточно того, что проверка `SELECT`-ом видит только снимок, а ограничения, как мы увидим, — нет.

### Почему одной базы тоже мало

Перенести все правила в базу и убрать их из домена тоже нельзя.

- **Хорошая ошибка — забота домена.** `NewUsername` возвращает `ValidationError` с полем `username` и причиной «длина от 3 до 32 символов», и HTTP-адаптер отдаёт её клиенту как `400 invalid_input` с указанием поля. Нарушение `CHECK` даёт код `23514` и имя ограничения: правило понятно программе, но не пользователю.
- **Ранний отказ.** Домен отвергает плохие данные до похода в базу, до транзакции и до дорогого шага. Регистрация проверяет имя **до** хеширования пароля Argon2id (урок 1.5.6), а не после.
- **Многие правила база не выразит.** «Автор сообщения — участник чата» требует заглянуть в другую таблицу, `CHECK` этого не умеет. «В группе не больше `GroupMaxMembers` = 200 участников» — правило о количестве строк. «Активность чата не идёт назад» — правило о переходе между состояниями, а не о самом состоянии.
- **Тесты.** Правила домена тестируются без базы, за миллисекунды (урок 1.6). Правило, которое живёт только в базе, проверит только интеграционный тест.

Отсюда рабочее правило, которому следует мессенджер: **домен проверяет всё, что может проверить, ради хороших ошибок и раннего отказа; база дублирует то, что можно выразить ограничением, ради гарантии при любом писателе; а то, что можно проверить только по многим строкам, держит только база**.

### Инварианты мессенджера

| правило | домен | база | почему так |
| --- | --- | --- | --- |
| формат и длина имени | `NewUsername` | `CHECK users_username_check` | одна строка: проверяют оба |
| имя уникально | — | `UNIQUE users_username_key` | нужно видеть все строки; домен не может |
| один личный чат на пару | `DirectKeyFor` вычисляет ключ пары | `UNIQUE chats_direct_key_key` | ключ — в домене, уникальность — в базе |
| у личного чата нет названия, у группы есть | конструкторы чата | парные `CHECK` в `chat.chats` | одна строка: оба |
| участник — существующий пользователь | порт `UserChecker` в `CreateChat` | `FOREIGN KEY members_user_id_fkey` | порт даёт ошибку со списком id, ключ закрывает гонку с удалением |
| автор — участник чата | `Chat.Post` | — | нужна другая таблица; гарантию даёт агрегат (урок 2.1) |
| не больше 200 участников в группе | `GroupMaxMembers` | — | правило о числе строк |
| `seq` без пропусков и повторов | агрегат выдаёт номер | `PRIMARY KEY (chat_id, seq)`, `CHECK (seq > 0)` | номер — в домене, «не повторяется» — в базе |
| повтор `client_message_id` | — | `UNIQUE messages_client_id_key` | естественный ключ повтора (урок 1.12) |

Обратите внимание на строку про уникальность имени: домен **не проверяет** её вовсе. Раньше это делало хранилище под мьютексом, а теперь Postgres-адаптер делает один `INSERT` без предварительного `SELECT` и ждёт ответа ограничения. Почему предварительный `SELECT` не просто лишний, а вредный, — в разделе «Типичные ошибки».

<Callout type="simplified">

Мы делим правила на «одна строка», «много строк одной таблицы» и «другие таблицы». Есть и четвёртый инструмент — триггер: функция, которую база вызывает при каждой вставке и которая может выполнить любой запрос, например посчитать участников группы. Урок 2.2 предложил добавить такой триггер «один дежурный на тему» и спросил, какая гонка остаётся. Ответ: та же «проверь, потом действуй». На уровне изоляции `READ COMMITTED` триггер двух одновременных транзакций видит только зафиксированные строки, и обе вставки проходят проверку. Закрыть окно может блокировка (`SELECT … FOR UPDATE` родительской строки) или уровень `SERIALIZABLE`. Обе темы — часть 2b. Ограничения `UNIQUE` и `EXCLUDE` устроены иначе, и ниже видно почему.

</Callout>

## <Part kind="theory" /> NOT NULL и CHECK: правила одной строки

### NOT NULL

`NOT NULL` запрещает отсутствие значения. Лаба, шаг 2:

```text
INSERT INTO users (username) VALUES (NULL);
psql:/lab/constraints.sql:47: ERROR:  null value in column "username" of relation "users" violates not-null constraint
DETAIL:  Failing row contains (3, null).
SQLSTATE 23502
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Код `23502` — `not_null_violation`. В схеме мессенджера `NOT NULL` стоит у каждого столбца, кроме двух: `title` и `direct_key` в `chat.chats`, и у каждого из них `NULL` значит «у этого вида чата этого атрибута нет». Начиная с PostgreSQL 18 `NOT NULL` хранится в каталоге `pg_constraint` как обычное ограничение со своим именем (release notes 18, раздел «Constraints»: «Store column NOT NULL specifications in pg_constraint»). Его можно назвать при создании и, как другие ограничения, добавить к существующей таблице с `NOT VALID`.

### CHECK

<T id="check-constraint">Ограничение CHECK</T> (CHECK constraint) проверяет булево выражение над значениями **одной** строки. Лаба, шаг 3:

```text
INSERT INTO users (username) VALUES ('Alice!');
psql:/lab/constraints.sql:54: ERROR:  new row for relation "users" violates check constraint "users_username_check"
DETAIL:  Failing row contains (4, Alice!).
SQLSTATE 23514
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Код `23514` — `check_violation`, и в ошибке есть имя ограничения. Это то преимущество перед `varchar(n)`, которое урок 2.3 показал на ошибке `22001` без имени.

У `CHECK` три свойства, которые нужно знать наизусть.

**Только эта строка.** Документация PostgreSQL 18 («Check Constraints»): «PostgreSQL does not support CHECK constraints that reference table data other than the new or updated row being checked». Подзапрос в `CHECK` запрещён синтаксически. Его можно спрятать в функцию, и база не помешает, но документация предупреждает: такое ограничение база проверяет только при записи **этой** строки и не перепроверит, когда изменится та, другая таблица. А восстановление дампа может упасть, потому что таблицы загружаются в другом порядке. Правило о других строках — это `UNIQUE`, `EXCLUDE`, `FOREIGN KEY` или домен.

**Выражение должно быть неизменным (immutable).** База проверяет `CHECK` один раз, при записи, и считает, что результат не изменится. `CHECK (expires_at > now())` сегодня пропустит строку, а завтра та же строка будет его нарушать, и никто об этом не узнает.

**`NULL` проходит.** `CHECK` отвергает строку, только если выражение **ложно**. Если оно `NULL`, строка проходит: документация говорит, что ограничение выполнено, «if the check expression evaluates to true or the null value». В мессенджере на этом держится `chats_title_check`:

```sql
CONSTRAINT chats_title_check CHECK (char_length(title) BETWEEN 1 AND 128),
```

У личного чата `title` равен `NULL`, `char_length(NULL)` — `NULL`, и ограничение личный чат пропускает. Это и нужно. А правило «у группы название обязательно» записано отдельным ограничением, потому что одно `chats_title_check` его не обеспечит.

### Парные CHECK: согласованность столбцов

Самые полезные `CHECK` связывают несколько столбцов строки. В `chat.chats` их два:

```sql
CONSTRAINT chats_direct_key_check CHECK ((kind = 'direct') = (direct_key IS NOT NULL)),
CONSTRAINT chats_title_kind_check CHECK ((kind = 'group') = (title IS NOT NULL)),
```

Первое читается так: «вид — личный» **равносильно** «ключ пары есть». Сравнение двух булевых значений через `=` записывает «тогда и только тогда» в одну строку. Личного чата без ключа и группы с ключом в базе не будет, а значит, `UNIQUE (direct_key)` действительно держит «один личный чат на пару» и не смешивает в себе группы. Здесь работает третье свойство `CHECK`: оба операнда никогда не бывают `NULL` (`kind` объявлен `NOT NULL`, а `IS NOT NULL` всегда даёт `true` или `false`), поэтому выражение не может «проскочить» через `NULL`.

### Константа домена и CHECK: как не разойтись

Урок 2.3 оставил вопрос: длины в `CHECK` повторяют константы домена (`UsernameMaxLen = 32`, `DisplayNameMaxLen = 64`, `TitleMaxLen = 128`, `TextMaxLen = 4096`), — как держать их согласованными? Одно и то же знание записано дважды, и правило DRY из стандарта кода требует назвать источник истины. Источник — домен: правило «имя до 32 символов» — бизнес-правило, а `CHECK` — его копия на случай записи в обход. Миграция мессенджера говорит это в шапке: «Длины в CHECK — те же, что константы домена (user/domain, chat/domain): база держит инвариант и тогда, когда строку пишет не наш код».

Что будет, если они разойдутся? Зависит от направления.

- **Домен шире базы** (в домене 64, в `CHECK` 32). Домен пропустит имя из 50 символов, база отвергнет его кодом `23514`, адаптер вернёт ошибку сервера, клиент получит `500`. Плохо, но громко: первый же тест на границу это поймает.
- **База шире домена** (в `CHECK` 128, в домене 64). База молча пропустит запись в обход приложения длиной 100 символов. Тишина длится до чтения: Postgres-адаптеры собирают сущность из строки через доменные конструкторы (`toUser` в `user/adapters/postgres`), и строка, которую домен не принимает, превратится в ошибку «повреждённые данные» при каждом её чтении.

Отсюда порядок изменения лимита: **расширяют сначала базу, потом домен; сужают сначала домен, потом базу**. Так в каждый момент база не уже домена. Сужение `CHECK` на большой таблице делают в два шага: `ALTER TABLE … ADD CONSTRAINT … NOT VALID` (новые строки проверяются сразу, старые — нет) и позже `VALIDATE CONSTRAINT`, который проверяет старые строки под более слабой блокировкой. Это частный случай миграций «расширь, потом сузь» (expand/contract) из урока 2.8 и части 2b.

<Callout type="simplified">

Сейчас согласованность держат комментарий в миграции и ревью: отдельного теста, который записывал бы через Postgres-адаптер имя ровно в `UsernameMaxLen` символов, в `storetest` нет. Промышленный способ закрыть это — тест границы в интеграционных тестах адаптера: сохранить сущность с максимальной длиной каждого поля. Он упадёт с `23514`, если `CHECK` окажется уже домена. Это упражнение в разделе «Что изменилось в мессенджере».

</Callout>

## <Part kind="theory" /> UNIQUE и PRIMARY KEY: правила многих строк

### Как UNIQUE закрывает гонку

`UNIQUE` запрещает две строки с равными значениями столбцов. PostgreSQL держит его уникальным <T id="index">индексом</T> (index) — B-деревом, тем же, что разбирали в уроке 2.3. И вот ключевое отличие от проверки в коде: **проверка и запись — одна операция внутри индекса**. Вставка находит в листе место для нового ключа и в том же месте смотрит, нет ли там равного. `LOCATION: _bt_check_unique, nbtinsert.c:666` из раздела «Проблема» — именно эта функция.

Окна между «проверил» и «записал» нет, потому что нет двух шагов. Что, если равный ключ уже вставлен, но его транзакция ещё не зафиксирована? Тогда вставка **ждёт**, чем закончится та транзакция: зафиксируется — ошибка `23505`, откатится — вставка проходит. Плеер ниже показывает это по шагам.

<Deep title="Что происходит внутри _bt_check_unique">

Сначала — как PostgreSQL хранит строки. Он не перезаписывает строку на месте: это <T id="mvcc">MVCC</T> (multiversion concurrency control, многоверсионность). `UPDATE` пишет **новую версию** строки, а старая остаётся на странице; `DELETE` ничего не стирает, а только помечает версию удалённой. В заголовке каждой версии два номера транзакций: `xmin` — кто её создал, `xmax` — кто её удалил, заменил новой или заблокировал (пока никто — ноль). Например, отправка сообщения обновляет строку чата в `chat.chats` (`last_seq`, `last_activity_at`), и после тысячи сообщений в таблице побывала тысяча версий этой строки. Какую из версий видит транзакция, решает её <T id="snapshot">снимок</T>: видна версия, чей `xmin` зафиксирован до снимка, а `xmax` пуст, откачен или ещё не зафиксирован к моменту снимка. Поэтому читатель не ждёт писателя: пока `UPDATE` не зафиксирован, остальные видят старую версию. Когда старую версию не видит уже ни один снимок, она становится <T id="dead-tuple">мёртвой строкой</T>: читать её никто не будет, но место на странице она занимает, пока её не уберёт фоновая очистка <T id="vacuum">`VACUUM`</T> (что она делает — урок 2.5; подробно — часть 2b).

Каждая запись индекса указывает на одну версию строки в таблице. В самом индексе `xmin` и `xmax` нет, поэтому индекс не знает, видна ли запись: он хранит и живые версии, и ещё не зафиксированные, и уже удалённые.

Поэтому `_bt_check_unique` в `src/backend/access/nbtree/nbtinsert.c` проходит по всем записям индекса с тем же ключом и для каждой спрашивает таблицу: жива ли эта версия? Ответа три.

- **Версия мертва** (её удалила зафиксированная транзакция или создала откаченная). Конфликта нет, смотрим дальше.
- **Версия жива и зафиксирована.** Конфликт: ошибка `23505`, текст `duplicate key value violates unique constraint`.
- **Версию создала или удаляет транзакция, которая ещё идёт.** Ответа пока нет. Функция возвращает номер этой транзакции вызывающей `_bt_doinsert`, а та отпускает блокировку страницы индекса и вызывает `XactLockTableWait` — ждёт окончания чужой транзакции. Потом поиск места и проверка начинаются заново.

Важная деталь: проверка видит **все** версии, а не только те, что видны снимку текущей транзакции. Поэтому `UNIQUE` работает на любом уровне изоляции, а проверка `SELECT`-ом в коде — нет. На `REPEATABLE READ` снимок транзакции B вообще не содержит строку, которую A вставила после начала B, и `SELECT` честно отвечает «свободно».

Цена ожидания — задержка: B стоит, пока A не закончит транзакцию. Поэтому транзакции, которые вставляют в уникальные индексы, должны быть короткими. Регистрация мессенджера считает хеш пароля **до** транзакции, и комментарий в `SignUp.Execute` называет причину: держать соединение и транзакцию сотни миллисекунд Argon2id — значит отнимать их у других запросов. А теперь видно и вторую причину: всё это время второй `alice` ждал бы у индекса.

</Deep>

### NULL в UNIQUE

По умолчанию два `NULL` в уникальном столбце **не равны** друг другу: `UNIQUE` пропустит сколько угодно строк с `NULL`. На этом держится `chats_direct_key_key`: у всех групп `direct_key` равен `NULL`, и групп может быть сколько угодно, а у личных чатов ключ есть и уникален. Комментарий в миграции так и говорит: «У групп direct_key NULL, а NULL в UNIQUE не конфликтуют друг с другом». С PostgreSQL 15 поведение можно поменять: `UNIQUE NULLS NOT DISTINCT` считает `NULL` равными, и второй `NULL` даст `23505`.

### PRIMARY KEY

<T id="primary-key">Первичный ключ</T> (primary key) — это `UNIQUE` плюс `NOT NULL`, один на таблицу. Нарушение уникальности первичного ключа приходит с тем же кодом `23505`, что и у любого `UNIQUE`, и различить их можно только по имени ограничения. Мессенджеру это важно: дубль `users_pkey` значит, что генератор id выдал повтор (сбой сервера), а дубль `users_username_key` — что имя занято (нормальный исход запроса). А у `auth.credentials` первичный ключ `user_id` **и есть** бизнес-правило «у пользователя одни учётные данные», и его нарушение `credentials_pkey` адаптер переводит в `ErrCredentialsExist`.

### Частичный и по выражению

Уникальный индекс может проверять не столбец, а **выражение**, и не всю таблицу, а строки, подходящие под **условие**. Лаба, шаг 7: адрес почты уникален без учёта регистра и только среди неудалённых строк.

```text
CREATE UNIQUE INDEX emails_address_active_key ON emails (lower(address)) WHERE deleted_at IS NULL;
CREATE INDEX
INSERT INTO emails (address) VALUES ('Alice@Example.com');
INSERT 0 1
INSERT INTO emails (address) VALUES ('alice@example.com');
psql:/lab/constraints.sql:104: ERROR:  duplicate key value violates unique constraint "emails_address_active_key"
DETAIL:  Key (lower(address))=(alice@example.com) already exists.
SQLSTATE 23505
UPDATE emails SET deleted_at = now() WHERE address = 'Alice@Example.com';
UPDATE 1
INSERT INTO emails (address) VALUES ('alice@example.com');
INSERT 0 1
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Пока первая строка жива, второй адрес отвергнут. После «мягкого удаления» (`deleted_at`, урок 2.6) строка выпала из условия `WHERE deleted_at IS NULL`, и тот же адрес можно занять снова.

Это не ограничение в строгом смысле, а уникальный индекс: `UNIQUE (…) WHERE …` в `CREATE TABLE` не пишется. Отсюда три следствия.

- **Имя в ошибке — имя индекса.** PostgreSQL сообщает «violates unique constraint "emails_address_active_key"», и в `ConstraintName` у `pgx` придёт имя индекса. Адаптер переводит ошибку тем же способом.
- **На него нельзя сослаться внешним ключом.** Документация («Foreign Keys»): внешний ключ ссылается на столбцы, которые «either are a primary key or form a unique constraint, or are columns from a non-partial unique index». Ни частичный, ни индекс по выражению не годятся.
- **`INSERT … ON CONFLICT` должен повторить выражение и условие**: `ON CONFLICT (lower(address)) WHERE deleted_at IS NULL DO NOTHING`. Иначе база не найдёт индекс, который должен решить конфликт.

## <Part kind="theory" /> FOREIGN KEY и ON DELETE

### Ссылка, которую проверяет база

<T id="foreign-key">Внешний ключ</T> (foreign key) — ограничение «значение этого столбца есть в таблице-родителе». Его нарушают с двух сторон связи, и лаба показывает обе (шаг 5):

```text
INSERT INTO members VALUES (99, 1);
psql:/lab/constraints.sql:73: ERROR:  insert or update on table "members" violates foreign key constraint "members_chat_id_fkey"
DETAIL:  Key (chat_id)=(99) is not present in table "chats".
SQLSTATE 23503
DELETE FROM users WHERE username = 'bob';
psql:/lab/constraints.sql:79: ERROR:  update or delete on table "users" violates foreign key constraint "members_user_id_fkey" on table "members"
DETAIL:  Key (id)=(2) is still referenced from table "members".
SQLSTATE 23503
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Оба нарушения — код `23503`, `foreign_key_violation`. Первое: участник несуществующего чата. Второе: удаление пользователя, на которого ещё ссылается участие в чате.

Как база это проверяет? Внешний ключ работает через системные триггеры. При вставке ребёнка триггер ищет строку родителя по его первичному ключу и **блокирует её в режиме `FOR KEY SHARE`** до конца транзакции. Поэтому родителя нельзя удалить между проверкой и фиксацией, и гонки «проверь, потом действуй» здесь тоже нет. При удалении родителя триггер ищет детей: `SELECT 1 FROM ONLY дети WHERE ссылка = $1 FOR KEY SHARE`. И здесь скрыта самая частая ошибка с внешними ключами.

### Индекс на ссылающемся столбце

Для поиска родителя индекс всегда есть: внешний ключ можно объявить только на первичный ключ или `UNIQUE`. А для поиска детей индекса может не быть. Документация PostgreSQL 18 («Foreign Keys»):

> Since a DELETE of a row from the referenced table or an UPDATE of a referenced column will require a scan of the referencing table for rows matching the old value, it is often a good idea to index the referencing columns too. Because this is not always needed, and there are many choices available on how to index, the declaration of a foreign key constraint does not automatically create an index on the referencing columns.

Проверим, сколько это стоит. Лаба, шаг 13: тысяча чатов, миллион сообщений со ссылкой на чат; удаляем чат 1001, у которого сообщений нет, — сначала без индекса по `big_messages.chat_id`, потом с ним. Подготовка (её вывод скрипт глушит `\set QUIET on`):

<<< @/../labs/2a/constraints.sql#fk-index-setup{sql}

Удаление без индекса и с ним:

```text
EXPLAIN (ANALYZE, COSTS OFF, BUFFERS OFF) DELETE FROM big_chats WHERE id = 1001;
                                           QUERY PLAN                                            
-------------------------------------------------------------------------------------------------
 Delete on big_chats (actual time=0.012..0.013 rows=0.00 loops=1)
   ->  Index Scan using big_chats_pkey on big_chats (actual time=0.004..0.005 rows=1.00 loops=1)
         Index Cond: (id = 1001)
         Index Searches: 1
 Planning Time: 0.065 ms
 Trigger for constraint big_messages_chat_id_fkey: time=22.441 calls=1
 Execution Time: 22.473 ms
(7 rows)

CREATE INDEX big_messages_chat_id_idx ON big_messages (chat_id);
CREATE INDEX
INSERT INTO big_chats VALUES (1001);
INSERT 0 1
EXPLAIN (ANALYZE, COSTS OFF, BUFFERS OFF) DELETE FROM big_chats WHERE id = 1001;
                                           QUERY PLAN                                            
-------------------------------------------------------------------------------------------------
 Delete on big_chats (actual time=0.011..0.011 rows=0.00 loops=1)
   ->  Index Scan using big_chats_pkey on big_chats (actual time=0.005..0.005 rows=1.00 loops=1)
         Index Cond: (id = 1001)
         Index Searches: 1
 Planning Time: 0.032 ms
 Trigger for constraint big_messages_chat_id_fkey: time=0.160 calls=1
 Execution Time: 0.178 ms
(7 rows)
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Само удаление строки чата — сотые доли миллисекунды в обоих случаях: строку нашёл первичный ключ. Всё время уходит в строку `Trigger for constraint big_messages_chat_id_fkey` — проверку детей. Без индекса она читает весь миллион сообщений: <N>22,441 мс</N>. С индексом — <N>0,160 мс</N>, <N>примерно в 140 раз</N> быстрее. Абсолютные миллисекунды зависят от машины, поэтому `scenario.sh` проверяет только соотношение: без индекса дольше больше чем в 10 раз. И это на таблице, целиком лежащей в памяти на tmpfs, и на **одном** удаляемом чате. Удаление тысячи чатов одной командой без индекса — тысяча полных просмотров. А `EXPLAIN` без `ANALYZE` эту цену не покажет вовсе: триггеры выполняются только при настоящем выполнении.

В схеме мессенджера у каждого внешнего ключа ссылающийся столбец проиндексирован, и не всегда отдельным индексом:

| внешний ключ | ссылающийся столбец | какой индекс его обслуживает |
| --- | --- | --- |
| `credentials_user_id_fkey` | `auth.credentials.user_id` | первичный ключ `credentials_pkey (user_id)` |
| `members_chat_id_fkey` | `chat.members.chat_id` | первый столбец первичного ключа `members_pkey (chat_id, user_id)` |
| `members_user_id_fkey` | `chat.members.user_id` | отдельный `members_user_id_idx (user_id)` |
| `messages_chat_id_fkey` | `chat.messages.chat_id` | первый столбец первичного ключа `messages_pkey (chat_id, seq)` |

Составной индекс годится для поиска по своему **первому** столбцу, но не по второму. Поэтому для `members.user_id` нужен свой индекс, и он нужен ещё и паттерну доступа «чаты пользователя» (урок 2.5).

### Что делать с детьми: ON DELETE

Внешний ключ задаёт и поведение при удалении родителя.

| действие | что происходит с детьми | когда уместно |
| --- | --- | --- |
| `NO ACTION` (по умолчанию) | удаление родителя — ошибка `23503`, если дети остались к моменту проверки | удаление родителя — бизнес-решение, которое кто-то должен принять явно |
| `RESTRICT` | то же, но проверка всегда немедленная | как `NO ACTION`, когда отложить проверку нельзя ни при каких условиях |
| `CASCADE` | дети удаляются вместе с родителем | у детей нет смысла без родителя, и их немного |
| `SET NULL`, `SET DEFAULT` | ссылка обнуляется или получает значение по умолчанию | связь необязательна, ребёнок живёт сам |

Разница между `NO ACTION` и `RESTRICT` тонкая. Документация: «The essential difference between these two choices is that NO ACTION allows the check to be deferred until later in the transaction, whereas RESTRICT does not». `CASCADE` из лабы, шаг 6:

```text
SELECT chat_id, count(*) AS members FROM members GROUP BY chat_id ORDER BY chat_id;
 chat_id | members 
---------+---------
       1 |       2
       2 |       1
(2 rows)

DELETE FROM chats WHERE id = 1;
DELETE 1
SELECT chat_id, count(*) AS members FROM members GROUP BY chat_id ORDER BY chat_id;
 chat_id | members 
---------+---------
       2 |       1
(1 row)
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Команда сообщила `DELETE 1`, а удалено три строки: чат и два его участника. Отчёт команды каскадных строк не считает.

Мессенджер выбирает действие для каждого ключа отдельно:

- **`auth.credentials → users.users` — `CASCADE`.** Связь 1:1, у учётных данных нет смысла без пользователя. Комментарий в миграции: «удалять их отдельным шагом — шанс забыть».
- **`chat.members → chat.chats` и `chat.messages → chat.chats` — `CASCADE`.** Участники и сообщения принадлежат чату: это агрегат из урока 1.2, и удалять его целиком естественно. Цена названа в «Типичных ошибках»: удаление большого чата — долгий каскад.
- **`chat.members → users.users` — `NO ACTION`.** Удаление пользователя сейчас вообще не поддерживается. Когда оно появится, нужно будет решить, что делать с его личными чатами, сообщениями в группах и требованиями приватности (урок 2.6). Пока решения нет, база запрещает случайное удаление: лаба показала это ошибкой «is still referenced from table "members"».
- **`chat.messages.author_id` — без внешнего ключа**, по причинам из урока 2.1: гарантию «автор существует» уже даёт проверка «автор — участник», а таблица сообщений — самая горячая.

Два внешних ключа из этого списка пересекают границу схем: из `auth` и из `chat` в `users`. Это осознанная связь контекстов на уровне базы, и её цену записывает ADR-0014 (раздел «Что изменилось в мессенджере»).

## <Part kind="theory" /> EXCLUDE: уникальность по любому оператору

`UNIQUE` запрещает две строки, у которых значения **равны**. <T id="exclusion-constraint">Ограничение исключения</T> (exclusion constraint) обобщает это на любой оператор: никакие две строки не должны давать `true` по **всем** указанным парам «столбец — оператор». Классический пример — диапазоны времени, которые не должны пересекаться: бронирование переговорной, смены дежурных, тарифы по периодам.

Мессенджеру такое ограничение пока не нужно, поэтому лаба (шаг 8) берёт правдоподобное будущее требование: «периоды тишины» участника в чате, когда уведомления не приходят. У одного участника в одном чате они не должны пересекаться:

```text
CREATE TABLE mutes (
    chat_id bigint    NOT NULL,
    user_id bigint    NOT NULL,
    during  tstzrange NOT NULL,
    CONSTRAINT mutes_no_overlap EXCLUDE USING gist (
        chat_id WITH =,
        user_id WITH =,
        during  WITH &&
    )
);
CREATE TABLE
INSERT INTO mutes VALUES (2, 1, '[2026-10-20 10:00+00, 2026-10-20 12:00+00)');
INSERT 0 1
INSERT INTO mutes VALUES (2, 2, '[2026-10-20 11:00+00, 2026-10-20 13:00+00)');
INSERT 0 1
INSERT INTO mutes VALUES (2, 1, '[2026-10-20 12:00+00, 2026-10-20 14:00+00)');
INSERT 0 1
INSERT INTO mutes VALUES (2, 1, '[2026-10-20 11:00+00, 2026-10-20 11:30+00)');
psql:/lab/constraints.sql:133: ERROR:  conflicting key value violates exclusion constraint "mutes_no_overlap"
DETAIL:  Key (chat_id, user_id, during)=(2, 1, ["2026-10-20 11:00:00+00","2026-10-20 11:30:00+00")) conflicts with existing key (chat_id, user_id, during)=(2, 1, ["2026-10-20 10:00:00+00","2026-10-20 12:00:00+00")).
SQLSTATE 23P01
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Ограничение читается так: «нет двух строк, у которых `chat_id` равны, **и** `user_id` равны, **и** `during` пересекаются (`&&`)». Второй участник в то же время — можно: `user_id` разные. Период 12:00–14:00 встык к 10:00–12:00 — тоже можно: `tstzrange` в записи `[a, b)` — полуоткрытый интервал, он включает начало и не включает конец, поэтому 12:00 принадлежит только второму периоду. А 11:00–11:30 внутри 10:00–12:00 — ошибка `23P01`, `exclusion_violation`.

Три технические детали. Ограничению нужен индекс, который понимает оператор `&&`. B-дерево умеет только «равно», «меньше» и «больше», поэтому здесь <T id="gist">GiST</T> (generalized search tree) — дерево, в узлах которого лежат обобщения ключей (например, охватывающий диапазон), и поиск спускается только в ветви, которые могут пересечься с искомым (`USING gist`). Сравнение обычных `bigint` через `=` в GiST даёт расширение `btree_gist` из contrib; лаба создаёт его в начале скрипта. И в PostgreSQL 18 появилась короткая запись для самого частого случая (release notes 18, раздел «Constraints»: «Allow the specification of non-overlapping PRIMARY KEY, UNIQUE, and foreign key constraints». Там же сказано, что для первичного ключа и `UNIQUE` это записывается словами `WITHOUT OVERLAPS`, а для внешних ключей — `PERIOD`): `UNIQUE (chat_id, user_id, during WITHOUT OVERLAPS)`. Под капотом это то же исключающее ограничение: лаба (шаг 15) вставляет те же 10:00–12:00 и 11:00–11:30 в таблицу `mutes_wo` с `UNIQUE … WITHOUT OVERLAPS` и получает «violates exclusion constraint "mutes_wo_key"» с тем же кодом `23P01`.

Почему это правило не проверить в коде? По той же причине, что и уникальность имени, только хуже. Две транзакции делают `SELECT … WHERE during && $1`, обе ничего не находят, обе вставляют. `SELECT … FOR UPDATE` не поможет: блокировать нечего, конфликтующей строки ещё нет. В коде окно закроет только уровень `SERIALIZABLE` (с повторами транзакций) или ручная блокировка, например `pg_advisory_xact_lock` на пару «чат, пользователь» — рекомендательная блокировка (advisory lock) по произвольному числу, которую база держит до конца транзакции, но смысл которой знает только код; писатель, который её не берёт, пройдёт мимо. `EXCLUDE` закрывает окно по тому же принципу, что и `UNIQUE`, хотя механизм другой: вставка сначала кладёт запись в индекс, а потом ищет в нём конфликтующие (`check_exclusion_or_unique_constraint` в `execIndexing.c`). Эта проверка тоже видит незафиксированные чужие записи и ждёт их исхода. Побочный эффект: две транзакции, вставившие конфликтующие строки одновременно, могут дождаться друг друга, и одна получит ошибку взаимоблокировки (deadlock, `40P01`) вместо `23P01`.

## <Part kind="theory" /> Отложенные ограничения

### Когда база проверяет ограничение

Обычное ограничение проверяется **сразу**. Документация `CREATE TABLE`: «A constraint that is not deferrable will be checked immediately after every command». Для `UNIQUE` и `PRIMARY KEY` PostgreSQL строже стандарта: он проверяет их не после команды, а **после каждой строки**. Раздел «Compatibility» той же страницы: «When a UNIQUE or PRIMARY KEY constraint is not deferrable, PostgreSQL checks for uniqueness immediately whenever a row is inserted or modified. The SQL standard says that uniqueness should be enforced only at the end of the statement».

Обычно разницы не видно. Но вот закреплённые сообщения чата с позициями 1 и 2, и нужно вставить новое в начало: сдвинуть все позиции на единицу (лаба, шаг 10):

```text
UPDATE pins SET position = position + 1 WHERE chat_id = 2;
psql:/lab/constraints.sql:179: ERROR:  duplicate key value violates unique constraint "pins_position_key"
DETAIL:  Key (chat_id, "position")=(2, 2) already exists.
SQLSTATE 23505
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Команда как целое правило не нарушает: после неё позиции 2 и 3. Но база обновляет строки по одной, и после первой строки две позиции 2 существуют одновременно. Проверка «после каждой строки» видит это промежуточное состояние.

### DEFERRABLE

<T id="deferrable-constraint">Откладываемое ограничение</T> (deferrable constraint) разрешает перенести проверку. У него два режима:

- **`DEFERRABLE INITIALLY IMMEDIATE`** — проверка после каждой **команды**, как требует стандарт. Внутри транзакции её можно отложить командой `SET CONSTRAINTS имя DEFERRED`;
- **`DEFERRABLE INITIALLY DEFERRED`** — по умолчанию проверка в момент `COMMIT`; командой `SET CONSTRAINTS имя IMMEDIATE` транзакция может вернуть немедленную проверку (и тогда отложенные до этого нарушения проверяются сразу).

Уже первый режим чинит сдвиг позиций (шаг 10, продолжение):

```text
ALTER TABLE pins ADD CONSTRAINT pins_position_key
    UNIQUE (chat_id, position) DEFERRABLE INITIALLY IMMEDIATE;
ALTER TABLE
UPDATE pins SET position = position + 1 WHERE chat_id = 2;
UPDATE 2
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Тот же `UPDATE` проходит: уникальность проверена после команды, а не посреди неё.

Второй классический случай — строки, которые ссылаются друг на друга, или ребёнок, который вставляется раньше родителя. Лаба, шаг 9: внешний ключ `members_chat_id_fkey` переведён в `DEFERRABLE INITIALLY IMMEDIATE`. Без `SET CONSTRAINTS` участник несуществующего чата отвергается сразу (`23503` на `INSERT`). А с ним:

```text
BEGIN;
BEGIN
SET CONSTRAINTS members_chat_id_fkey DEFERRED;
SET CONSTRAINTS
INSERT INTO members VALUES (3, 1);
INSERT 0 1
INSERT INTO chats VALUES (3, 'Релиз');
INSERT 0 1
COMMIT;
COMMIT
BEGIN;
BEGIN
SET CONSTRAINTS members_chat_id_fkey DEFERRED;
SET CONSTRAINTS
INSERT INTO members VALUES (4, 1);
INSERT 0 1
COMMIT;
psql:/lab/constraints.sql:161: ERROR:  insert or update on table "members" violates foreign key constraint "members_chat_id_fkey"
DETAIL:  Key (chat_id)=(4) is not present in table "chats".
SQLSTATE 23503
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. В первой транзакции участник вставлен раньше чата, и к `COMMIT` всё согласовано. Во второй чат так и не появился, и ошибка пришла не на `INSERT`, а на `COMMIT`: транзакция откатилась целиком.

### Что можно отложить

Только четыре вида: `UNIQUE`, `PRIMARY KEY`, `FOREIGN KEY` и `EXCLUDE`. Документация `CREATE TABLE`, параметр `DEFERRABLE`: «Only UNIQUE, PRIMARY KEY, EXCLUDE, and REFERENCES (foreign key) constraints accept this clause. NOT NULL and CHECK constraints are not deferrable». Лаба проверяет это (шаг 11):

```text
ALTER TABLE users ADD CONSTRAINT users_username_short CHECK (length(username) <= 16) DEFERRABLE;
psql:/lab/constraints.sql:196: ERROR:  CHECK constraints cannot be marked DEFERRABLE
LINE 1: ...ers_username_short CHECK (length(username) <= 16) DEFERRABLE...
                                                             ^
SQLSTATE 0A000
ALTER TABLE chats ADD CONSTRAINT chats_title_not_null NOT NULL title DEFERRABLE;
psql:/lab/constraints.sql:200: ERROR:  NOT NULL constraints cannot be marked DEFERRABLE
LINE 1: ...DD CONSTRAINT chats_title_not_null NOT NULL title DEFERRABLE...
                                                             ^
SQLSTATE 0A000
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Код `0A000` — `feature_not_supported`. Логика понятна: `CHECK` и `NOT NULL` — правила одной строки, и у одной строки не бывает «промежуточного состояния посреди транзакции», которое станет верным позже. Если строка неверна, её нужно записать верной, а не ждать `COMMIT`.

### Цена и почему мессенджер ничего не откладывает

1. **Ошибка приходит не там.** Отложенное нарушение всплывает на `COMMIT`, а не в команде, которая его вызвала. В мессенджере `COMMIT` выполняет `platform/pgtx` — реализация порта `Transactor`, которая ничего не знает о домене и именах ограничений. Перевести `23503` в доменную ошибку там нельзя, а адаптер, который знает, что значит `members_chat_id_fkey`, к этому моменту уже вернул управление. Отложенные ограничения ломают правило «ошибку переводит адаптер, выполнивший запрос».
2. **`ON CONFLICT` с ними не работает.** Отложенное ограничение нельзя использовать как арбитра конфликта: на попытку PostgreSQL 18 отвечает «ON CONFLICT does not support deferrable unique constraints/exclusion constraints as arbiters» (лаба, шаг 14: код `55000`). А на `INSERT … ON CONFLICT` держится резервация ключа идемпотентности.
3. **Проверка медленнее.** Тот же раздел документации о `DEFERRABLE INITIALLY IMMEDIATE` у уникальности: «Be aware that this can be significantly slower than immediate uniqueness checking».

Ни одного из двух случаев, ради которых откладывают, у мессенджера нет. Позиций, которые сдвигаются массово, нет. А «чат и его участники» вставляются одной командой: <T id="cte">CTE</T> (common table expression, именованный подзапрос `WITH … AS (…)` в начале команды; в PostgreSQL в нём может стоять и `INSERT`) вставляет строку чата, основной `INSERT` — участников, и внешний ключ `members → chats`, проверяемый после команды, видит строку из `CTE` (регион `create-chat` в `chat/adapters/postgres/store.go`, разбор — в уроке 2.9). Поэтому в миграции ни одно ограничение не объявлено `DEFERRABLE`.

## <Part kind="theory" /> От ошибки базы к доменной ошибке

### Код SQLSTATE и имя ограничения

<T id="sqlstate">SQLSTATE</T> — пятисимвольный код ошибки из стандарта SQL, расширенный PostgreSQL. Первые два символа — класс: `23` — нарушение ограничения целостности, `25` — неверное состояние транзакции, `40` — откат транзакции (с ним встретимся в части 2b, когда дойдём до `SERIALIZABLE`). Коды этого урока:

| код | имя | что нарушено |
| --- | --- | --- |
| `23502` | `not_null_violation` | `NOT NULL` |
| `23514` | `check_violation` | `CHECK` |
| `23505` | `unique_violation` | `UNIQUE`, `PRIMARY KEY`, уникальный индекс |
| `23503` | `foreign_key_violation` | `FOREIGN KEY` с любой стороны |
| `23P01` | `exclusion_violation` | `EXCLUDE`, `WITHOUT OVERLAPS` |
| `0A000` | `feature_not_supported` | например, `DEFERRABLE` у `CHECK` |
| `25P02` | `in_failed_sql_transaction` | команда в оборванной транзакции (ниже) |

Почему программа смотрит на код, а не на текст сообщения? Документация PostgreSQL 18, приложение A: «Applications that need to know which error condition has occurred should usually test the error code, rather than looking at the textual error message. The error codes are less likely to change across PostgreSQL releases, and also are not subject to change due to localization of error messages». Текст зависит от настройки `lc_messages` сервера: с русской локалью `duplicate key value violates unique constraint` придёт по-русски. И текст меняется между версиями.

Код говорит, **что** нарушено, но не **какое** правило: у `users.users` два уникальных ограничения, `users_pkey` и `users_username_key`, и оба дают `23505`. Поэтому второй ключ перевода — имя ограничения. Миграция мессенджера задаёт все имена явно, и её шапка объясняет зачем: «адаптеры переводят ошибку 23505/23503/23514 в доменную по pgconn.PgError.ConstraintName, и имя — часть контракта, а не то, что Postgres придумает сам». Имя ограничения по умолчанию (`users_username_key`) PostgreSQL строит из таблицы и столбцов, обрезает до 63 байт и при совпадении добавляет число. Код, который на него опирается, зависит от того, что не написано ни в одном файле.

### Как это выглядит в pgx

Драйвер `pgx` возвращает ошибку сервера как `*pgconn.PgError`. В нём те же поля, что `psql` печатал в разделе «Проблема»: `Code`, `ConstraintName`, `SchemaName`, `TableName`, `Detail`. Ошибку обычно оборачивают по дороге (`fmt.Errorf("…: %w", err)`), поэтому достают её через `errors.As`. Сравните хрупкий и правильный способы:

<!-- go-check -->
```go
package snippet

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Хрупко: текст зависит от lc_messages и версии сервера, а «duplicate key»
// одинаков у первичного ключа и у имени пользователя.
func takenByText(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}

// Правильно: код SQLSTATE — что нарушено, имя ограничения — какое правило.
func takenByCode(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "users_username_key"
}
```

В мессенджере этот разбор живёт в пакете `platform/pgerr`, одной функцией (код — в разделе «Код»). Функция принимает имя ограничения и возвращает `false` для любого другого: «дубль первичного ключа, например, значит сбой генератора id, а не "имя занято", и переводить его в ту же доменную ошибку нельзя».

Готовые константы кодов есть в пакете `github.com/jackc/pgerrcode` (`pgerrcode.UniqueViolation`), и многие проекты им пользуются. Мессенджеру нужен один код, и одна именованная константа с комментарием проще, чем ещё одна зависимость.

### Что переводить в доменную ошибку, а что нет

Не каждое нарушение ограничения — доменная ошибка. Мессенджер переводит **только те, что бывают при правильной работе**: когда два правильных запроса пришли одновременно и один из них проиграл.

| ограничение | когда нарушается | во что переводится |
| --- | --- | --- |
| `users_username_key` | два запроса регистрируют одно имя | `user/domain.ErrUsernameTaken` → `409 username_taken` |
| `credentials_pkey` | учётные данные для пользователя уже есть | `auth/domain.ErrCredentialsExist` |
| `chats_direct_key_key` | два запроса создают личный чат одной пары | `chat/domain.ErrDirectChatExists` |
| `messages_client_id_key` | клиент повторил отправку | `chat/domain.ErrDuplicateMessage` → повтор или `ErrClientMessageIDReused` |
| любой `CHECK` (`23514`) | домен пропустил то, что база не принимает | ошибка сервера |
| `users_pkey`, `chats_pkey` (`23505`) | генератор id выдал повтор | ошибка сервера |
| `members_user_id_fkey` (`23503`) | участника удалили между проверкой и вставкой | ошибка сервера |

Нижние строки — это ситуации, когда домен **уже проверил** правило, а база не согласна. Значит, сломано что-то в сервере: константа домена разошлась с `CHECK`, генератор id сбоит, данные удалили в обход. Клиенту честно ответить `500`, а в журнал — вся цепочка с `%w`, где `*pgconn.PgError` сохранит код, имя ограничения и `Detail`. Превратить такую ошибку в «неверные данные» значило бы обвинить клиента в ошибке сервера и спрятать её от дежурного.

Где `23503` стал бы доменной ошибкой? Если бы API позволял добавить участника по id без предварительной проверки, то `members_user_id_fkey` и был бы ответом «нет такого пользователя». Мессенджер проверяет участников портом `UserChecker` до записи: так ошибка перечисляет **все** отсутствующие id, а не только первый, на котором упал ключ. Внешний ключ остаётся страховкой от гонки с удалением.

### Ошибка ограничения обрывает транзакцию

Есть деталь, которая меняет код use case'а. Ошибка внутри транзакции PostgreSQL не просто отменяет команду: она **обрывает всю транзакцию**, и до `ROLLBACK` база не выполнит в ней ни одной команды. Лаба, шаг 12 (`alice` в таблице уже есть):

```text
BEGIN;
BEGIN
INSERT INTO users (username) VALUES ('alice');
psql:/lab/constraints.sql:208: ERROR:  duplicate key value violates unique constraint "users_username_key"
DETAIL:  Key (username)=(alice) already exists.
SQLSTATE 23505
SELECT username FROM users WHERE username = 'alice';
psql:/lab/constraints.sql:213: ERROR:  current transaction is aborted, commands ignored until end of transaction block
SQLSTATE 25P02
ROLLBACK;
ROLLBACK
SELECT username FROM users WHERE username = 'alice';
 username 
----------
 alice
(1 row)
```

<Run dir="labs" cmd="docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&amp;1'" needs="docker" before="`docker compose -f 2a/compose.yaml up -d --wait`" />

машина та же. Тот же `SELECT` в оборванной транзакции — ошибка `25P02`, после `ROLLBACK` — обычный ответ.

Это ровно ситуация повторной отправки сообщения. `SendMessage` пишет сообщение в транзакции (порт `Transactor`). Если `client_message_id` уже был, `INSERT` нарушает `messages_client_id_key`, и прочитать сохранённое сообщение **в той же транзакции** нельзя. В in-memory хранилище модуля 1 ошибка приносила сохранённое сообщение с собой (`DuplicateMessageError.Existing`), а Postgres-адаптер этого не может. Поэтому use case читает его **после отката**, отдельным портом `MessageByClientIDFinder`, и сам сравнивает текст. Тот же ключ и тот же текст — повтор, `200`. Тот же ключ и другой текст — `ErrClientMessageIDReused`, `409`. Код — в разделе «Код», регион `send-saved`.

Есть два способа обойти обрыв, и оба мессенджер знает.

- **Точка сохранения (savepoint).** `SAVEPOINT s` перед рискованной командой и `ROLLBACK TO SAVEPOINT s` после ошибки откатывают только её, а транзакция продолжается. В `pgx` вложенный `tx.Begin(ctx)` делает именно это. Цена — лишние обращения к базе на каждую команду и подтранзакции, у которых в PostgreSQL есть известные проблемы производительности под нагрузкой: GitLab в 2021 году описал, как подтранзакции на реплике приводили к деградации, и убрал их из кода. `pgtx` мессенджера вложенный `WithinTx` точками сохранения не делает: он переиспользует внешнюю транзакцию.
- **`INSERT … ON CONFLICT DO NOTHING`.** Конфликт перестаёт быть ошибкой: команда просто ничего не вставляет и не обрывает транзакцию. Так устроена резервация ключа идемпотентности в `platform/idempotency/postgres`. Для сообщений этот способ **не годится**, и причина поучительна. Запись сообщения — одна команда: `CTE` с `UPDATE chat.chats SET last_seq = last_seq + 1 …` и `INSERT` сообщения с этим номером. С `ON CONFLICT DO NOTHING` повтор выполнил бы `UPDATE`, а `INSERT` пропустил: номер `last_seq` израсходован, сообщения с ним нет, и в ленте появилась бы дыра. Ошибка `23505` откатывает команду целиком вместе с `UPDATE`. Комментарий в `store.go` говорит это прямо: «если INSERT нарушит уникальность client_message_id, откатится и UPDATE, и счётчик не убежит вперёд сообщений».

## <Part kind="viz" /> Визуализация: плеер constraint-race

Плеер показывает две регистрации одного имени `alice` с двух экземпляров сервиса. Переключатель сверху выбирает, кто следит за уникальностью: «Проверка в коде» (`SELECT`, потом `INSERT`, ограничения нет) или «Ограничение UNIQUE». Карточки экземпляров A и B показывают последний оператор SQL, его результат и HTTP-ответ. Полоса слева у карточки — состояние экземпляра: работает, ждёт чужой транзакции, ответил или получил ошибку. Ниже — таблица `users.users` и, во втором варианте, путь ошибки к клиенту B.

<ConstraintRace />

Что смотреть:

1. **Окно гонки.** В варианте «Проверка в коде» на шаге 3 у обоих экземпляров результат «0 строк: имя свободно». Обе проверки верны в момент проверки. Это и есть окно «проверь, потом действуй»: в нём уже две проверки и пока ни одной записи.
2. **Тихий дубль.** На последних шагах того же варианта в таблице две строки `alice`, обе подсвечены цветом ошибки, у обоих клиентов `201 Created`, а плашка сверху сообщает, что правило нарушено. Ошибки не получил никто: база не знала о правиле.
3. **Незафиксированная строка.** Переключитесь на «Ограничение UNIQUE». На шаге 2 строка `alice` в таблице курсивом с пометкой «не зафиксирована: видит только A». Другие сеансы её не видят, но уникальный индекс видит.
4. **Ожидание.** На шаге 3 экземпляр B в состоянии «ждёт исхода чужой транзакции». Его `INSERT` нашёл в индексе `alice` от незавершённой транзакции A и не может ответить, пока A не закончит. Это ожидание `XactLockTableWait` из блока «Что происходит внутри _bt_check_unique».
5. **Перевод ошибки.** На шагах 5–8 подсвечиваются звенья пути ошибки: `*pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"}` в `pgx`, `user/domain.ErrUsernameTaken` в Postgres-адаптере, `auth/domain.ErrUsernameTaken` в мосте `userbridge` и `409 application/problem+json` в HTTP-адаптере. Каждое звено знает только соседей, а use case не изменился ни строкой.

<Callout type="simplified">

Плеер сжимает регистрацию до одной строки `users.users`. В мессенджере в той же транзакции записываются и учётные данные (`auth.credentials`), а хеш пароля считается до неё. Время между шагами условное: на настоящей машине окно гонки — доли миллисекунды, и потому `storetest` устраивает 100 забегов по 16 одновременных регистраций, а не один. Если бы транзакция A откатилась, B не получил бы ошибку: вставка прошла бы, и имя досталось бы B. Этот исход плеер не показывает.

</Callout>

Вопросы к визуализации:

1. В варианте «Проверка в коде» поменяйте уровень изоляции обеих транзакций на `REPEATABLE READ`. Изменится ли исход? А на `SERIALIZABLE`?
2. На шаге 3 варианта с `UNIQUE` транзакция A, вместо того чтобы зафиксироваться, откатилась из-за ошибки при записи учётных данных. Что получит клиент B?
3. Почему Postgres-адаптер проверяет не только код `23505`, но и имя `users_username_key`? Какой ещё `23505` бывает у этой таблицы?

<SelfCheck question="Ответы">

1. На `REPEATABLE READ` — нет: снимок каждой транзакции не содержит незафиксированную строку соседа, оба `SELECT` вернут «свободно», оба `INSERT` пройдут (ограничения нет). На `SERIALIZABLE` PostgreSQL отследит, что каждая транзакция прочитала то, что записала другая, и одну из них откатит с кодом `40001` (`serialization_failure`). Это закрывает гонку, но ценой повторов транзакций и для **каждого** писателя: ручной `INSERT` на `READ COMMITTED` по-прежнему вставит дубль. Подробно — часть 2b.
2. `INSERT` в B дождётся исхода A, увидит, что версия `alice` от A мертва (транзакция откатилась), и пройдёт. Клиент B получит `201 Created`, клиент A — ошибку своей регистрации. Имя досталось B, и это правильно: A так и не стал пользователем.
3. У `users.users` два уникальных ограничения: `users_pkey (id)` и `users_username_key (username)`. Дубль первичного ключа значит, что генератор id выдал повтор, — это сбой сервера, и переводить его в «имя занято» нельзя: клиент сменит имя и снова получит отказ, а сбой генератора останется незамеченным.

</SelfCheck>

## <Part kind="code" /> Код: ограничения в миграции и перевод ошибок

### Ограничения в миграции

Схема мессенджера — файл `messenger/migrations/00001_init.sql`. Как он применяется и почему миграция — отдельный шаг, разберём в уроке 2.8, а здесь — ограничения. Все имена явные, длины совпадают с константами домена.

<<< @/../messenger/migrations/00001_init.sql#users{sql}

Учётные данные: первичный ключ — он же внешний, связь 1:1, `CASCADE`.

<<< @/../messenger/migrations/00001_init.sql#credentials{sql}

Чаты: парные `CHECK` и `UNIQUE` на ключе пары, который у групп `NULL`.

<<< @/../messenger/migrations/00001_init.sql#chats{sql}

Участники: первичный ключ из двух внешних, межсхемный ключ на `users.users` без `ON DELETE`, то есть `NO ACTION`. И индекс по `user_id` — тот, которого нет у второго столбца первичного ключа.

<<< @/../messenger/migrations/00001_init.sql#members{sql}

<<< @/../messenger/migrations/00001_init.sql#members-user-index{sql}

Сообщения: естественный ключ повтора `messages_client_id_key` и отсутствие внешнего ключа на автора.

<<< @/../messenger/migrations/00001_init.sql#messages{sql}

### Разбор ошибки: platform/pgerr

Одна функция на весь мессенджер: код и имя ограничения, `errors.As` сквозь обёртки.

<<< @/../messenger/internal/platform/pgerr/pgerr.go#is-unique-violation{go}

Табличный тест рядом с ней проверяет обёрнутую ошибку, другое ограничение с тем же кодом, другой код с тем же именем, «ошибку» с именем ограничения в тексте и `nil` (`pgerr_test.go`).

### Перевод в адаптерах

Каждый адаптер знает имена **своих** ограничений и переводит нарушение в ошибку своего порта. Пользователи:

<<< @/../messenger/internal/user/adapters/postgres/errors.go#create-user-error{go}

Учётные данные — здесь `23505` первичного ключа и есть бизнес-правило:

<<< @/../messenger/internal/auth/adapters/postgres/errors.go{go}

Чаты и сообщения:

<<< @/../messenger/internal/chat/adapters/postgres/errors.go#chat-errors{go}

### Повтор сообщения после отката

`SendMessage` получает от адаптера `ErrDuplicateMessage`, когда транзакция попытки уже откатилась, и читает сохранённое сообщение вне её:

<<< @/../messenger/internal/chat/usecase/send_message.go#send-saved{go}

### SQL лабы constraints.sql

Выводы этих шагов вы видели в теории; полностью — `labs/2a/results/constraints.txt`.

<<< @/../labs/2a/constraints.sql#schema{sql}

<<< @/../labs/2a/constraints.sql#unique{sql}

<<< @/../labs/2a/constraints.sql#partial-unique{sql}

<<< @/../labs/2a/constraints.sql#exclude{sql}

<<< @/../labs/2a/constraints.sql#without-overlaps{sql}

<<< @/../labs/2a/constraints.sql#deferrable-fk{sql}

<<< @/../labs/2a/constraints.sql#deferrable-unique{sql}

<<< @/../labs/2a/constraints.sql#deferrable-unique-fix{sql}

<<< @/../labs/2a/constraints.sql#deferrable-on-conflict{sql}

<<< @/../labs/2a/constraints.sql#aborted-transaction{sql}

<<< @/../labs/2a/constraints.sql#aborted-transaction-next{sql}

<<< @/../labs/2a/constraints.sql#aborted-transaction-rollback{sql}

<<< @/../labs/2a/constraints.sql#fk-without-index{sql}

<<< @/../labs/2a/constraints.sql#fk-with-index{sql}

## <Part kind="practice" /> Что изменилось в мессенджере

После урока 2.3 мессенджер переехал на PostgreSQL: появились миграция, Postgres-адаптеры, транзакции на уровне use case и пул соединений. Всё это урок за уроком разбирают 2.5 (запросы и индексы), 2.8 (миграции) и 2.9 (адаптеры, транзакции, пул, пробы). К этому уроку относятся ограничения и перевод их ошибок:

```text
messenger/
├── migrations/00001_init.sql             # все ограничения с явными именами; регионы для урока
├── docs/adr/0014-схема-и-контексты.md    # новый: схема на контекст, межсхемные FK к users.users
└── internal/
    ├── platform/pgerr/                   # IsUniqueViolation: код 23505 + имя ограничения
    ├── user/adapters/postgres/errors.go  # users_username_key → ErrUsernameTaken
    ├── auth/adapters/postgres/errors.go  # credentials_pkey → ErrCredentialsExist
    └── chat/
        ├── adapters/postgres/errors.go   # chats_direct_key_key → ErrDirectChatExists,
        │                                 # messages_client_id_key → ErrDuplicateMessage
        └── usecase/send_message.go       # повтор читается после отката (MessageByClientIDFinder)
```

### ADR-0014: схема и контексты в базе

Урок 2.1 разложил таблицы по схемам PostgreSQL — по одной на ограниченный контекст — и пообещал записать решение здесь, вместе с решениями об ограничениях. ADR-0014 (`messenger/docs/adr/0014-схема-и-контексты.md`) фиксирует:

- **схема на контекст** в одной базе: `users`, `auth`, `chat` и `platform` для технических таблиц;
- **межсхемные внешние ключи — только к `users.users`**, и их ровно два: из `auth.credentials` и из `chat.members`. Идентичность пользователя — общий язык всех контекстов;
- **цену**: схемы нельзя мигрировать и вынести в другую базу независимо, удаление пользователя затрагивает чужие контексты, регистрация — транзакция поверх двух контекстов (подробно — ADR-0015 в уроке 2.9);
- **первый шаг при выделении сервиса**: снять межсхемный внешний ключ и заменить его проверкой через порт, как `UserChecker` уже делает в `CreateChat`;
- **ограничения как контракт**: имена ограничений явные, ошибки переводятся по коду и имени, у каждого внешнего ключа ссылающийся столбец проиндексирован, `ON DELETE` выбирается для каждого ключа отдельно;
- **отвергнутые варианты**: всё в схеме `public` и база на контекст.

Запуск лабы — из каталога `labs/`:

```bash
docker compose -f 2a/compose.yaml up -d --wait
docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&1'
docker compose -f 2a/compose.yaml down -v
```

`bash 2a/scenario.sh` прогоняет `constraints.sql` и проверяет, что в выводе ровно 16 ошибок с кодами `23502`, `23514`, `23505`, `23503`, `23P01`, `0A000`, `25P02` и `55000`, что отложенный внешний ключ пропустил участника чата 3 и что проверка внешнего ключа без индекса дольше, чем с индексом, больше чем в 10 раз (только соотношение: миллисекунды зависят от машины). Перевод ошибок в мессенджере проверяют `storetest`-батареи против настоящего PostgreSQL (testcontainers, урок 2.9): `testUsernameTaken` и `testConcurrentSameUsername` в `user`, `testSaveTwice` и `testConcurrentSave` в `auth`, `testDirectUnique`, `testDuplicateClientMessageID` и `testConcurrentDuplicateClientMessageID` в `chat`.

Попробуйте сами:

- Добавьте в `user/usecase/storetest` тест границы: сохранить пользователя с именем ровно `UsernameMaxLen` символов и отображаемым именем ровно `DisplayNameMaxLen` символов, и прочитать его. Поменяйте в копии миграции `char_length(username) BETWEEN 3 AND 32` на `… AND 16` и убедитесь, что тест падает с `23514`, а in-memory адаптер его проходит.
- В базе лабы удалите индекс `members_user_id_idx` из схемы мессенджера (её создаёт `messenger-explain.sql`) и повторите опыт с `EXPLAIN (ANALYZE)` для `DELETE FROM users.users` — какой триггер теперь стоит дорого?
- Вставьте в `mutes_wo` из шага 15 период встык, 12:00–14:00, и период другого участника. Пройдут ли они, как в `mutes_no_overlap`?
- Задайте в сеансе `psql` `SET lc_messages = 'C'` и `SET lc_messages = 'ru_RU.UTF-8'` (если локаль есть в образе) и сравните текст одной и той же ошибки `23505`. Что из полей ошибки не изменилось?

Состояние репозитория после урока — тег `m02-l04`.

## <Part kind="mistakes" /> Типичные ошибки

<Callout type="mistake">

**Проверка уникальности `SELECT`-ом перед `INSERT`.** «Найти имя; не нашли — вставить» — гонка «проверь, потом действуй»: два запроса одновременно проходят проверку, и без ограничения в базе оба вставляют. С ограничением предварительный `SELECT` не нужен для корректности — ответ всё равно даёт `23505`, — и его можно оставить разве что как оптимизацию (не считать дорогой хеш для занятого имени). Тогда в коде два пути к одной ошибке, и тестировать нужно оба. Мессенджер делает один `INSERT` и переводит ошибку.

**Разбор текста ошибки.** `strings.Contains(err.Error(), "duplicate key")` ломается от `lc_messages = 'ru_RU.UTF-8'`, от новой версии сервера и от второго уникального ограничения в той же таблице. Различайте по `pgconn.PgError.Code` и `ConstraintName`, через `errors.As`.

**Ограничения без явных имён.** Имя, которое придумал PostgreSQL, — деталь реализации: оно собрано из имён таблицы и столбцов на момент создания, обрезается на длинных именах и получает число при совпадении. Пересоздали таблицу или переставили столбцы в ограничении — получили другое имя, а переименовали таблицу — старое имя осталось и больше ей не соответствует. Если адаптер переводит ошибки по имени, имя — часть контракта схемы, и его пишут в миграции явно, как константу в коде.

**Все `23505` — одна доменная ошибка.** Дубль первичного ключа и дубль имени пользователя — разные события: первое — сбой сервера, второе — нормальный исход запроса. Перевод только по коду превращает сбой генератора id в «имя занято».

**Внешний ключ без индекса на ссылающемся столбце.** Объявление внешнего ключа не создаёт ни одного индекса: на стороне родителя индекс обязан уже быть (первичный ключ или `UNIQUE`), а на стороне детей его нет, пока вы его не создадите. Удаление родителя ищет детей, и без индекса это полный просмотр таблицы детей на **каждую** удалённую строку: в лабе (шаг 13) 22,441 мс вместо 0,160 мс на одном чате и миллионе сообщений. `EXPLAIN` без `ANALYZE` этого не покажет. Для каждого внешнего ключа проверьте, что ссылающийся столбец — первый в каком-то индексе.

**`ON DELETE CASCADE` «везде».** Каскад удобен, пока дети — часть родителя и их немного. «Везде» он превращает одно случайное `DELETE` в удаление половины базы, а цепочку `users → chats → messages` — в долгую транзакцию, которая держит блокировки на миллионах строк, пишет гигабайты WAL и отправляет их на реплики. И каскад через границу контекстов удаляет чужие данные без ведома их владельца. Выбирайте действие для каждого ключа: `CASCADE` — для частей агрегата, `NO ACTION` — там, где удаление родителя должно быть явным решением. Большие удаления делают порциями в фоне, а не одним каскадом.

**`CHECK`, который «проверяет» `NULL`.** `CHECK (char_length(title) BETWEEN 1 AND 128)` пропускает `NULL`: ограничение отвергает только ложь. Если значение обязательно, нужен `NOT NULL` или парное условие, как `chats_title_kind_check`.

**`DEFERRABLE` «на всякий случай».** Отложенная проверка переносит ошибку на `COMMIT`, где её некому перевести, ломает `ON CONFLICT` и медленнее немедленной. Откладывайте конкретное ограничение ради конкретного сценария (сдвиг позиций, взаимные ссылки), а не всё подряд.

</Callout>

## <Part kind="tradeoff" /> Компромисс: правило в двух местах

<Callout type="tradeoff">

**Только домен.** Хорошие ошибки, тесты без базы, одно место правды. Платите корректностью: правило держится, пока писатель один и процесс один. Второй экземпляр, скрипт и ручной SQL его не знают, а уникальность и непересечение по многим строкам домен не проверит вообще.

**Только база.** Гарантия при любом писателе и гонке. Платите качеством ошибок (клиенту `23514` ничего не говорит), поздним отказом (после хеша пароля и начала транзакции) и тестами, которым нужна база. И база выразит не всё: правила о других таблицах, о числе строк, о переходах между состояниями ей недоступны.

**Оба — выбор мессенджера.** Домен проверяет всё, что может, ради ошибок и раннего отказа; база дублирует правила одной строки и единолично держит правила многих строк. Платите дублированием знания: длина имени записана в константе домена и в `CHECK`, и они должны меняться в правильном порядке (сначала расширяют базу, сначала сужают домен). Это осознанное исключение из DRY: одна копия — правило, вторая — страховка от писателя, который правила не знает.

**Цена ограничений вообще.** Каждое ограничение — работа при каждой записи: уникальный индекс растёт и обновляется, внешний ключ ищет родителя и блокирует его строку. Именно поэтому у `chat.messages.author_id` внешнего ключа нет: гарантия уже есть, а таблица самая горячая. Межсхемный внешний ключ ещё и связывает контексты: его придётся снять, прежде чем схему можно будет вынести (ADR-0014).

</Callout>

## <Part kind="selfcheck" /> Самопроверка

<SelfCheck question="Почему мьютекс, который держал уникальность имени в in-memory хранилище модуля 1, не годится, когда мессенджер работает на PostgreSQL? Назовите три вида писателей, которых он не видит.">

Мьютекс живёт в памяти одного процесса и упорядочивает только горутины этого процесса. С Postgres писателей больше: второй экземпляр того же сервиса за балансировщиком; другие программы, которые пишут в ту же базу (скрипт миграции данных, задача в планировщике, админка); ручной SQL дежурного в `psql`. Все они проходят через базу, поэтому правило «имя уникально» должно жить в базе — ограничением `UNIQUE`, которое проверяет каждую запись.

</SelfCheck>

<SelfCheck question="Почему UNIQUE закрывает гонку двух регистраций, а SELECT перед INSERT — нет, даже на уровне REPEATABLE READ? Что делает вторая вставка, если первая ещё не зафиксирована?">

У `SELECT` и `INSERT` два шага, и между ними окно, в которое помещается чужая запись. Снимок `REPEATABLE READ` не содержит незафиксированных (и зафиксированных после его начала) строк соседа, поэтому `SELECT` честно отвечает «свободно». `UNIQUE` проверяет и записывает одной операцией в уникальном индексе, и эта проверка видит **все** версии ключа, а не только видимые снимку. Если равный ключ вставлен незавершённой транзакцией, вторая вставка ждёт её исхода (`_bt_check_unique` находит её запись, `_bt_doinsert` ждёт в `XactLockTableWait`): фиксация — ошибка `23505`, откат — вставка проходит.

</SelfCheck>

<SelfCheck question="В chat.chats ограничение chats_title_check CHECK (char_length(title) BETWEEN 1 AND 128). Почему его пропускает личный чат с title = NULL, и как схема всё же гарантирует, что у группы название есть?">

`CHECK` отвергает строку, только если выражение ложно, а `char_length(NULL)` — `NULL`, и `NULL BETWEEN 1 AND 128` — тоже `NULL`. Ограничение считается выполненным. Обязательность названия у группы держит отдельное парное ограничение `chats_title_kind_check CHECK ((kind = 'group') = (title IS NOT NULL))`: оба операнда всегда `true` или `false`, и группа без названия, как и личный чат с названием, его нарушат.

</SelfCheck>

<SelfCheck question="Коллега предлагает переводить в pgerr любой 23505 в ErrUsernameTaken, «чтобы не возиться с именами ограничений», а заодно 23514 — в ошибку валидации 400. Что не так с каждым предложением?">

`23505` у таблицы пользователей дают два ограничения: `users_username_key` и `users_pkey`. Дубль первичного ключа значит сбой генератора id. Если перевести его в «имя занято», клиент сменит имя и снова получит отказ, а сбой сервера никто не увидит. `23514` в нашем коде значит, что домен пропустил строку, которую база не принимает: константа домена разошлась с `CHECK` или запись шла в обход домена. Это ошибка сервера, и `400` обвинил бы клиента в нашей ошибке. Правильно: доменная ошибка — только для нарушений, которые бывают при правильной работе (гонка двух правильных запросов), по коду **и** имени; остальное — ошибка сервера с `%w`, чтобы код и `Detail` попали в журнал.

</SelfCheck>

<SelfCheck question="SendMessage при повторе client_message_id получает от адаптера ErrDuplicateMessage и читает сохранённое сообщение отдельным портом MessageByClientIDFinder, после отката. Почему не прочитать его сразу, в той же транзакции? И почему адаптер не использует ON CONFLICT DO NOTHING, раз он не обрывает транзакцию?">

Ошибка ограничения обрывает транзакцию PostgreSQL: до `ROLLBACK` любая команда в ней получает `25P02 in_failed_sql_transaction`, так что прочитать сообщение в ней нельзя (можно было бы через точку сохранения, но это лишние обращения к базе и подтранзакции). `ON CONFLICT DO NOTHING` не годится, потому что запись сообщения — одна команда: `UPDATE chat.chats SET last_seq = last_seq + 1 …` в `CTE` и `INSERT` сообщения. При конфликте `UPDATE` выполнился бы, а `INSERT` — нет: номер израсходован, сообщения с ним нет, в ленте дыра. Ошибка `23505` откатывает всю команду вместе с `UPDATE`.

</SelfCheck>

<SelfCheck question="Почему отложить можно UNIQUE, PRIMARY KEY, FOREIGN KEY и EXCLUDE, но не CHECK и NOT NULL? И почему мессенджер не откладывает ни одного ограничения, хотя чат и его участники вставляются в одном сценарии?">

Отложенная проверка нужна, когда **набор строк** проходит через промежуточное неверное состояние: два уникальных значения меняются местами, ребёнок вставлен раньше родителя. Все четыре откладываемых вида — правила о нескольких строках. `CHECK` и `NOT NULL` — правила одной строки: её не нужно ждать до `COMMIT`, её нужно сразу записать верной; PostgreSQL отвечает на попытку кодом `0A000`. Мессенджеру откладывать незачем: чат и участники вставляются **одной командой** (`CTE` с чатом и `INSERT` участников), а внешний ключ проверяется после команды и видит строку из `CTE`. Отложенное ограничение принесло бы ошибку на `COMMIT` в `pgtx`, где её нельзя перевести в доменную, и сломало бы `ON CONFLICT`.

</SelfCheck>

<SelfCheck question="Для каждого внешнего ключа мессенджера назовите индекс, который обслуживает поиск детей при удалении родителя. Что случится, если для chat.members.user_id такого индекса не будет?">

`credentials_user_id_fkey` — первичный ключ `credentials_pkey (user_id)`; `members_chat_id_fkey` — первый столбец `members_pkey (chat_id, user_id)`; `messages_chat_id_fkey` — первый столбец `messages_pkey (chat_id, seq)`; `members_user_id_fkey` — отдельный `members_user_id_idx (user_id)`, потому что `user_id` — второй столбец первичного ключа, и по нему одному составной индекс не ищет. Без него удаление пользователя (когда оно появится) полностью просматривало бы `chat.members` на каждого удалённого: в лабе (шаг 13) такой просмотр миллиона строк — 22,441 мс против 0,160 мс с индексом. И тот же индекс нужен запросу «чаты пользователя».

</SelfCheck>

## <Part kind="sources" /> Источники

<Callout type="source">

- Документация PostgreSQL 18: [Constraints](https://www.postgresql.org/docs/18/ddl-constraints.html) — `CHECK` (только эта строка, `NULL` проходит, неизменность), `UNIQUE` и `NULLS NOT DISTINCT`, внешние ключи (индекс на ссылающихся столбцах, `NO ACTION` против `RESTRICT`, ссылка только на первичный ключ, ограничение уникальности или нечастичный уникальный индекс), исключающие ограничения; [CREATE TABLE](https://www.postgresql.org/docs/18/sql-createtable.html) — `DEFERRABLE` (только `UNIQUE`, `PRIMARY KEY`, `EXCLUDE`, `REFERENCES`), «checked immediately after every command», раздел «Compatibility» о немедленной проверке уникальности и цене `DEFERRABLE INITIALLY IMMEDIATE`, `WITHOUT OVERLAPS`; [SET CONSTRAINTS](https://www.postgresql.org/docs/18/sql-set-constraints.html); [ALTER TABLE](https://www.postgresql.org/docs/18/sql-altertable.html) — `NOT VALID` и `VALIDATE CONSTRAINT`; [INSERT](https://www.postgresql.org/docs/18/sql-insert.html) — `ON CONFLICT` и выбор индекса-арбитра; [Unique Indexes](https://www.postgresql.org/docs/18/indexes-unique.html) и [Partial Indexes](https://www.postgresql.org/docs/18/indexes-partial.html); [btree_gist](https://www.postgresql.org/docs/18/btree-gist.html); [Range Types](https://www.postgresql.org/docs/18/rangetypes.html) — полуоткрытые диапазоны и исключающие ограничения; [PostgreSQL Error Codes](https://www.postgresql.org/docs/18/errcodes-appendix.html) — классы и коды, совет проверять код, а не текст.
- PostgreSQL 18 Release Notes, раздел «Constraints»: `NOT NULL` в `pg_constraint`, `NOT VALID` для `NOT NULL`, `WITHOUT OVERLAPS` и `PERIOD`: [release-18](https://www.postgresql.org/docs/18/release-18.html).
- Исходники PostgreSQL 18.6 (тег `REL_18_6`): [`src/backend/access/nbtree/nbtinsert.c`](https://github.com/postgres/postgres/blob/REL_18_6/src/backend/access/nbtree/nbtinsert.c) — `_bt_check_unique` находит запись незавершённой транзакции, `_bt_doinsert` ждёт её через `XactLockTableWait`; [`src/backend/executor/execIndexing.c`](https://github.com/postgres/postgres/blob/REL_18_6/src/backend/executor/execIndexing.c) — `check_exclusion_or_unique_constraint` для `EXCLUDE`; [`src/backend/utils/adt/ri_triggers.c`](https://github.com/postgres/postgres/blob/REL_18_6/src/backend/utils/adt/ri_triggers.c) — проверки внешнего ключа запросами `… FOR KEY SHARE`.
- `github.com/jackc/pgx/v5` v5.11.0: [`pgconn/errors.go`](https://github.com/jackc/pgx/blob/v5.11.0/pgconn/errors.go) — `PgError` с полями `Code`, `ConstraintName`, `SchemaName`, `TableName`, `Detail`; [`tx.go`](https://github.com/jackc/pgx/blob/v5.11.0/tx.go) — вложенный `Begin` как точка сохранения. Пакет констант кодов — [github.com/jackc/pgerrcode](https://github.com/jackc/pgerrcode).
- GitLab, [«Why we spent the last month eliminating PostgreSQL subtransactions»](https://about.gitlab.com/blog/2021/09/29/why-we-spent-the-last-month-eliminating-postgresql-subtransactions/), 2021 — цена подтранзакций (`SAVEPOINT`) под нагрузкой.
- Michael J. Hernandez, «Database Design for Mere Mortals», 4th ed., Addison-Wesley, 2020 — бизнес-правила и ограничения целостности, правила на уровне поля и связи.
- Bill Karwin, «SQL Antipatterns, Volume 1», Pragmatic Bookshelf, 2022 — глава «Keyless Entry» (схема без внешних ключей), «ID Required» (суррогатный ключ без `UNIQUE` на естественном).
- Dimitri Fontaine, «The Art of PostgreSQL», 2nd ed., 2020 — ограничения как часть модели данных, `EXCLUDE` с диапазонами.
- Курс: урок [1.2](../01-go-service/02-domain) — инварианты и агрегат; [1.12](../01-go-service/12-idempotency) — `client_message_id` и повтор; [2.1](./01-modeling) — схемы по контекстам, межсхемные ключи, `author_id` без внешнего ключа; [2.2](./02-normalization) — правила, которые держит база, а какие код; [2.3](./03-keys-types) — `text` + `CHECK` против `varchar(n)`, `UNIQUE` без учёта регистра.
- Код: `messenger/migrations/00001_init.sql` (регионы `users`, `credentials`, `chats`, `members`, `members-user-index`, `messages`); `messenger/internal/platform/pgerr/pgerr.go` (регион `is-unique-violation`), `pgerr_test.go`; `messenger/internal/{user,auth,chat}/adapters/postgres/errors.go` (регионы `create-user-error`, `chat-errors`); `messenger/internal/chat/usecase/send_message.go` (регион `send-saved`); `messenger/docs/adr/0014-схема-и-контексты.md`; `labs/2a/constraints.sql` (регионы `schema`, `unique`, `partial-unique`, `exclude`, `deferrable-fk`, `deferrable-unique`, `deferrable-unique-fix`, `deferrable-on-conflict`, `without-overlaps`, `aborted-transaction`, `aborted-transaction-next`, `aborted-transaction-rollback`, `fk-index-setup`, `fk-without-index`, `fk-with-index`), вывод `labs/2a/results/constraints.txt`, `machine.txt`; сцена `course/viz/simulations/constraint-race.ts`.

</Callout>
