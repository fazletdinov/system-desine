package httpapi

import (
	"time"

	"messenger/internal/user/domain"
)

// DTO (data transfer object) — форма данных на проводе. Отдельные типы с тегами json,
// а не доменные структуры: домен меняется по своим причинам, контракт API — по своим.

type registerUserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type userResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

func toUserResponse(u domain.User) userResponse {
	return userResponse{
		ID:          u.ID().String(),
		Username:    u.Username().String(),
		DisplayName: u.DisplayName().String(),
		CreatedAt:   u.CreatedAt().UTC().Format(time.RFC3339Nano),
	}
}
