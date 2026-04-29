package usecase

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/puzakov/gophermart-loyalty-exam/internal/auth"
	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
	"github.com/puzakov/gophermart-loyalty-exam/internal/storage/postgres"
)

type AuthUsecase struct {
	users  *postgres.UsersRepo
	tokens *auth.TokenManager
}

type Tokens struct {
	AccessToken string
}

func NewAuthUsecase(users *postgres.UsersRepo, tokens *auth.TokenManager) *AuthUsecase {
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
	if postgres.IsUniqueViolation(err) {
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
	if postgres.IsNoRows(err) {
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
