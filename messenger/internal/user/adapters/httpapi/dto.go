package httpapi

import (
	"messenger/internal/user/adapters/httpapi/gen"
	"messenger/internal/user/domain"
)

// DTO (data transfer object) — форма данных на проводе. Типы сгенерированы из
// api/openapi.yaml в пакет gen; здесь — только явный перевод домена в них.

func toUser(u domain.User) gen.User {
	return gen.User{
		Id:          u.ID().String(),
		Username:    u.Username().String(),
		DisplayName: u.DisplayName().String(),
		CreatedAt:   u.CreatedAt().UTC(),
	}
}
