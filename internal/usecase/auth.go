package usecase

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/puzakov/gophermart-loyalty-exam/internal/auth"
	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
)

type UsersStore interface {
	Create(ctx context.Context, login, passwordHash string) (userID int64, err error)
	GetByLogin(ctx context.Context, login string) (userID int64, passwordHash string, err error)
}

type AuthUsecase struct {
	users  UsersStore
	tokens *auth.TokenManager
}

type Tokens struct {
	AccessToken string
}

func NewAuthUsecase(users UsersStore, tokens *auth.TokenManager) *AuthUsecase {
	return &AuthUsecase{users: users, tokens: tokens}
}

func (u *AuthUsecase) Register(ctx context.Context, login, password string, now time.Time) (Tokens, error) {
	if login == "" || password == "" {
		return Tokens{}, domain.ErrInvalidRequest
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, err
	}
	userID, err := u.users.Create(ctx, login, string(hash))
	if err == domain.ErrConflict {
		return Tokens{}, domain.ErrConflict
	}
	if err != nil {
		return Tokens{}, err
	}
	return u.issueTokens(userID, now)
}

func (u *AuthUsecase) Login(ctx context.Context, login, password string, now time.Time) (Tokens, error) {
	if login == "" || password == "" {
		return Tokens{}, domain.ErrInvalidRequest
	}
	userID, ph, err := u.users.GetByLogin(ctx, login)
	if err == domain.ErrNotFound {
		return Tokens{}, domain.ErrUnauthorized
	}
	if err != nil {
		return Tokens{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(ph), []byte(password)); err != nil {
		return Tokens{}, domain.ErrUnauthorized
	}
	return u.issueTokens(userID, now)
}

func (u *AuthUsecase) issueTokens(userID int64, now time.Time) (Tokens, error) {
	access, _, err := u.tokens.NewAccessToken(userID, now)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access}, nil
}
