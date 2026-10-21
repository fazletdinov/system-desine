package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
	"messenger/internal/platform/pgtx"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, errors.New("postgres: nil-пул")
	}
	return &Store{pool: pool}, nil
}

// #region create-chat
// insertChat — чат и его участники одним оператором: CTE вставляет строку чата, основной
// INSERT — по строке участника на каждый элемент $9. Оператор атомарен и без транзакции
// use case'а: сорвётся вставка участника — не останется и чата. Внешний ключ
// members → chats проверяется в конце оператора и видит строку из CTE.
const insertChat = `
WITH chat AS (
    INSERT INTO chat.chats (id, kind, title, direct_key, last_seq, version, created_at, last_activity_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    RETURNING id
)
INSERT INTO chat.members (chat_id, user_id)
SELECT chat.id, member FROM chat, unnest($9::uuid[]) AS member`

// #endregion create-chat

func (s *Store) CreateChat(ctx context.Context, chat *domain.Chat) error {
	snap := chat.Snapshot()
	var title, direct *string // NULL: у личного чата нет названия, у группы — ключа пары
	if !snap.Title.IsZero() {
		t := snap.Title.String()
		title = &t
	}
	if key, ok := chat.DirectKey(); ok {
		direct = &key
	}
	_, err := pgtx.Q(ctx, s.pool).Exec(ctx, insertChat,
		snap.ID.String(), string(snap.Kind), title, direct, snap.LastSeq, snap.Version,
		snap.CreatedAt, snap.LastActivityAt, userIDStrings(snap.Members))
	if err != nil {
		return createChatError(err)
	}
	return nil
}

// selectChat — строка чата с участниками. ORDER BY user_id важен: RestoreChat ждёт
// участников отсортированными (поиск участника — двоичный), а порядок uuid в PostgreSQL —
// побайтовый, тот же, что у строк id в нижнем регистре в домене.
const selectChat = `
SELECT c.id, c.kind, c.title, c.last_seq, c.version, c.created_at, c.last_activity_at,
       ARRAY(SELECT m.user_id FROM chat.members m WHERE m.chat_id = c.id ORDER BY m.user_id)
FROM chat.chats c`

func (s *Store) FindChat(ctx context.Context, id domain.ChatID) (*domain.Chat, error) {
	return scanChat(pgtx.Q(ctx, s.pool).QueryRow(ctx, selectChat+` WHERE c.id = $1`, id.String()))
}

// FindDirectChat ищет по direct_key — тому же ключу пары, что держит UNIQUE.
func (s *Store) FindDirectChat(ctx context.Context, a, b domain.UserID) (*domain.Chat, error) {
	return scanChat(pgtx.Q(ctx, s.pool).QueryRow(ctx, selectChat+` WHERE c.direct_key = $1`, domain.DirectKeyFor(a, b)))
}

// #region append
// appendMessage — новое состояние чата и сообщение одним оператором. UPDATE выдаёт номер
// (last_seq = last_seq + 1) только при той версии, которую прочитал use case, —
// оптимистическая блокировка; INSERT берёт номер из RETURNING. Строку чата UPDATE
// держит заблокированной до конца транзакции: второй писатель того же чата ждёт, а
// после фиксации первого видит новую версию и не обновляет ничего (урок 2.2).
//
// Один оператор, а не два, — чтобы запись была атомарной и без транзакции use case'а:
// если INSERT нарушит уникальность client_message_id, откатится и UPDATE, и счётчик не
// убежит вперёд сообщений. В транзакции SendMessage оператор выполняется в ней.
const appendMessage = `
WITH chat AS (
    UPDATE chat.chats
    SET last_seq = last_seq + 1, version = version + 1, last_activity_at = $3
    WHERE id = $1 AND version = $2
    RETURNING last_seq
)
INSERT INTO chat.messages (chat_id, seq, id, author_id, client_message_id, text, sent_at)
SELECT $1, chat.last_seq, $4, $5, $6, $7, $8 FROM chat
RETURNING seq`

// #endregion append

const chatExists = `SELECT EXISTS (SELECT 1 FROM chat.chats WHERE id = $1)`

func (s *Store) AppendMessage(ctx context.Context, chat *domain.Chat, msg domain.Message, expectedVersion int64) error {
	q := pgtx.Q(ctx, s.pool)
	var seq int64
	err := q.QueryRow(ctx, appendMessage,
		chat.ID().String(), expectedVersion, chat.LastActivityAt(),
		msg.ID().String(), msg.AuthorID().String(), msg.ClientID().String(), msg.Text().String(), msg.SentAt(),
	).Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		// UPDATE не нашёл строку: либо чата нет, либо версия ушла вперёд.
		return s.missingOrConcurrent(ctx, q, chat.ID())
	}
	if err != nil {
		return appendMessageError(err)
	}
	if seq != msg.Seq() {
		// Версия совпала, а номер — нет: last_seq и version разошлись, кто-то писал в
		// обход агрегата. В транзакции use case'а запись откатится.
		return fmt.Errorf("postgres: чат %s: база выдала seq %d, агрегат — %d", chat.ID(), seq, msg.Seq())
	}
	return nil
}

func (s *Store) missingOrConcurrent(ctx context.Context, q pgtx.Querier, id domain.ChatID) error {
	var exists bool
	if err := q.QueryRow(ctx, chatExists, id.String()).Scan(&exists); err != nil {
		return fmt.Errorf("postgres: проверка чата: %w", err)
	}
	if !exists {
		return domain.ErrChatNotFound
	}
	return domain.ErrConcurrentUpdate
}

const selectMessage = `SELECT chat_id, seq, id, author_id, client_message_id, text, sent_at FROM chat.messages`

// FindMessageByClientID читает по тому же ключу, что держит UNIQUE messages_client_id_key,
// — его индекс и обслуживает запрос.
func (s *Store) FindMessageByClientID(ctx context.Context, chatID domain.ChatID, author domain.UserID, clientID domain.ClientMessageID) (domain.Message, error) {
	row := pgtx.Q(ctx, s.pool).QueryRow(ctx,
		selectMessage+` WHERE chat_id = $1 AND author_id = $2 AND client_message_id = $3`,
		chatID.String(), author.String(), clientID.String())
	m, err := scanMessage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, usecase.ErrMessageNotFound
	}
	return m, err
}

// #region list-before
// listMessagesBefore — один диапазон первичного ключа (chat_id, seq), читаемый с конца.
const listMessagesBefore = selectMessage + `
WHERE chat_id = $1 AND seq < $2
ORDER BY seq DESC
LIMIT $3`

// #endregion list-before

func (s *Store) ListMessagesBefore(ctx context.Context, chatID domain.ChatID, beforeSeq int64, limit int) ([]domain.Message, error) {
	// Отменённый запрос — ошибка и при limit < 1: порт сначала смотрит на ctx, как memory.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if limit < 1 {
		return []domain.Message{}, nil
	}
	if beforeSeq <= 0 {
		beforeSeq = math.MaxInt64 // «с самого нового» — та же граница, что «до бесконечности»
	}
	rows, err := pgtx.Q(ctx, s.pool).Query(ctx, listMessagesBefore, chatID.String(), beforeSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: история чата: %w", err)
	}
	page, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Message, error) { return scanMessage(row) })
	if err != nil {
		return nil, fmt.Errorf("postgres: история чата: %w", err)
	}
	return page, nil
}

// #region list-chats
// Список чатов участника по убыванию (last_activity_at, id) — keyset-пагинация: курсор
// сравнивается со строкой как пара, и равные времена разрешает id. Какие индексы
// обслуживают запрос — labs/2a/results/messenger-explain.txt (урок 2.5).
const (
	listChatsFirst = selectChat + `
JOIN chat.members me ON me.chat_id = c.id
WHERE me.user_id = $1
ORDER BY c.last_activity_at DESC, c.id DESC
LIMIT $2`
	listChatsAfter = selectChat + `
JOIN chat.members me ON me.chat_id = c.id
WHERE me.user_id = $1 AND (c.last_activity_at, c.id) < ($3, $4)
ORDER BY c.last_activity_at DESC, c.id DESC
LIMIT $2`
)

// #endregion list-chats

func (s *Store) ListChatsByMember(ctx context.Context, member domain.UserID, after *usecase.ChatPosition, limit int) ([]*domain.Chat, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if limit < 1 {
		return []*domain.Chat{}, nil
	}
	sql, args := listChatsFirst, []any{member.String(), limit}
	if after != nil {
		sql, args = listChatsAfter, append(args, after.LastActivityAt, after.ID.String())
	}
	rows, err := pgtx.Q(ctx, s.pool).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: чаты участника: %w", err)
	}
	chats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Chat, error) { return scanChat(row) })
	if err != nil {
		return nil, fmt.Errorf("postgres: чаты участника: %w", err)
	}
	return chats, nil
}

// scanChat читает строку selectChat и собирает агрегат конструкторами домена.
func scanChat(row pgx.Row) (*domain.Chat, error) {
	var (
		rawID, rawKind      string
		rawTitle            *string
		snap                domain.ChatSnapshot
		createdAt, activity time.Time
		rawMembers          []string
	)
	err := row.Scan(&rawID, &rawKind, &rawTitle, &snap.LastSeq, &snap.Version, &createdAt, &activity, &rawMembers)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrChatNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: чтение чата: %w", err)
	}
	if snap.ID, err = domain.ParseChatID(rawID); err != nil {
		return nil, corrupt("чат", rawID, err)
	}
	if snap.Kind, err = domain.ParseKind(rawKind); err != nil {
		return nil, corrupt("чат", rawID, err)
	}
	if rawTitle != nil {
		if snap.Title, err = domain.NewTitle(*rawTitle); err != nil {
			return nil, corrupt("чат", rawID, err)
		}
	}
	snap.Members = make([]domain.UserID, len(rawMembers))
	for i, raw := range rawMembers {
		if snap.Members[i], err = domain.ParseUserID(raw); err != nil {
			return nil, corrupt("чат", rawID, err)
		}
	}
	snap.CreatedAt, snap.LastActivityAt = createdAt.UTC(), activity.UTC()
	return domain.RestoreChat(snap), nil
}

// scanMessage читает строку selectMessage. pgx.ErrNoRows возвращает как есть: что значит
// «нет строки», решает вызывающий.
func scanMessage(row pgx.Row) (domain.Message, error) {
	var (
		rawChat, rawID, rawAuthor, rawClient, rawText string
		snap                                          domain.MessageSnapshot
	)
	err := row.Scan(&rawChat, &snap.Seq, &rawID, &rawAuthor, &rawClient, &rawText, &snap.SentAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Message{}, err
	}
	if err != nil {
		return domain.Message{}, fmt.Errorf("postgres: чтение сообщения: %w", err)
	}
	if snap.ChatID, err = domain.ParseChatID(rawChat); err != nil {
		return domain.Message{}, corrupt("сообщение", rawID, err)
	}
	if snap.ID, err = domain.ParseMessageID(rawID); err != nil {
		return domain.Message{}, corrupt("сообщение", rawID, err)
	}
	if snap.AuthorID, err = domain.ParseUserID(rawAuthor); err != nil {
		return domain.Message{}, corrupt("сообщение", rawID, err)
	}
	if snap.ClientID, err = domain.ParseClientMessageID(rawClient); err != nil {
		return domain.Message{}, corrupt("сообщение", rawID, err)
	}
	if snap.Text, err = domain.NewText(rawText); err != nil {
		return domain.Message{}, corrupt("сообщение", rawID, err)
	}
	return domain.RestoreMessage(snap), nil
}

// corrupt — строка, которую записали в обход домена. Только текст причины, без %w:
// ValidationError конструктора — не ошибка данных клиента, и HTTP-адаптер не должен
// ответить на неё 400.
func corrupt(what, id string, err error) error {
	return fmt.Errorf("postgres: %s %q не проходит проверки домена: %s", what, id, err.Error())
}

func userIDStrings(ids []domain.UserID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
