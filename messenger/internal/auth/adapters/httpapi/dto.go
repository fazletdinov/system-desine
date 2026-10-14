package httpapi

import (
	"messenger/internal/auth/adapters/httpapi/gen"
	"messenger/internal/auth/usecase"
)

// toUser переводит профиль глазами auth в DTO ответа регистрации. Схема User — та же,
// что у GET /v1/users/{id}: клиент получает одинаковый профиль от обоих контекстов.
func toUser(p usecase.Profile) gen.User {
	return gen.User{
		Id:          p.ID.String(),
		Username:    p.Username,
		DisplayName: p.DisplayName,
		CreatedAt:   p.CreatedAt.UTC(),
	}
}
