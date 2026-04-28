package usecase

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/puzakov/gophermart-loyalty-exam/internal/auth"
	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
	"github.com/puzakov/gophermart-loyalty-exam/internal/storage/postgres"
)

type AuthUsecase struct {
	users   *postgres.UsersRepo
	refresh *postgres.RefreshTokensRepo
	tokens  *auth.TokenManager
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
}

func NewAuthUsecase(users *postgres.UsersRepo, refresh *postgres.RefreshTokensRepo, tokens *auth.TokenManager) *AuthUsecase {
	return &AuthUsecase{users: users, refresh: refresh, tokens: tokens}
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
	return u.issueTokens(ctx, userID, now)
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
	return u.issueTokens(ctx, userID, now)
}

func (u *AuthUsecase) Refresh(ctx context.Context, rawRefreshToken string, now time.Time) (Tokens, error) {
	if rawRefreshToken == "" {
		return Tokens{}, domain.ErrUnauthorized
	}
	hash := auth.HashRefreshToken(rawRefreshToken)
	userID, err := u.refresh.Consume(ctx, hash, now)
	if postgres.IsNoRows(err) {
		return Tokens{}, domain.ErrUnauthorized
	}
	if err != nil {
		return Tokens{}, err
	}
	return u.issueTokens(ctx, userID, now)
}

func (u *AuthUsecase) issueTokens(ctx context.Context, userID int64, now time.Time) (Tokens, error) {
	access, _, err := u.tokens.NewAccessToken(userID, now)
	if err != nil {
		return Tokens{}, err
	}
	refreshRaw, refreshHash, refreshExp, err := u.tokens.NewRefreshToken(now)
	if err != nil {
		return Tokens{}, err
	}
	if err := u.refresh.Store(ctx, userID, refreshHash, refreshExp); err != nil {
		// Very unlikely collision; retry once.
		if errors.Is(err, domain.ErrConflict) || postgres.IsUniqueViolation(err) {
			refreshRaw, refreshHash, refreshExp, err = u.tokens.NewRefreshToken(now)
			if err != nil {
				return Tokens{}, err
			}
			if err := u.refresh.Store(ctx, userID, refreshHash, refreshExp); err != nil {
				return Tokens{}, err
			}
		} else {
			return Tokens{}, err
		}
	}
	return Tokens{AccessToken: access, RefreshToken: refreshRaw}, nil
}
