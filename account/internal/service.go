package internal

import (
	"context"
	"errors"

	"github.com/Tuananh165art/GoshopX/account/models"
	"github.com/Tuananh165art/GoshopX/pkg/auth"
	"github.com/Tuananh165art/GoshopX/pkg/crypt"
)

type Service interface {
	Register(ctx context.Context, name, email, password string) (string, error)
	Login(ctx context.Context, email, password string) (string, error)
	GetAccount(ctx context.Context, id uint64) (*models.Account, error)
	GetAccounts(ctx context.Context, skip uint64, take uint64) ([]*models.Account, error)
}

type accountService struct {
	repository Repository
}

func NewService(r Repository) Service {
	return &accountService{r}
}

func (service accountService) Register(ctx context.Context, name, email, password string) (string, error) {
	_, err := service.repository.GetAccountByEmail(ctx, email)
	if err == nil {
		return "", errors.New("account already exists")
	}

	hashedPass, err := crypt.HashPassword(password)
	if err != nil {
		return "", err
	}
	acc := models.Account{
		Name:     name,
		Email:    email,
		Password: hashedPass,
	}
	account, err := service.repository.PutAccount(ctx, acc)
	if err != nil {
		return "", err
	}
	token, err := auth.GenerateToken(account.ID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (service accountService) Login(ctx context.Context, email, password string) (string, error) {
	account, err := service.repository.GetAccountByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	err = crypt.VerifyPassword(password, account.Password)
	if err != nil {
		return "", err
	}

	token, err := auth.GenerateToken(account.ID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (service accountService) GetAccount(ctx context.Context, id uint64) (*models.Account, error) {
	return service.repository.GetAccountByID(ctx, id)
}

func (service accountService) GetAccounts(ctx context.Context, skip uint64, take uint64) ([]*models.Account, error) {
	if take > 100 || (skip == 0 && take == 0) {
		take = 100
	}

	return service.repository.ListAccounts(ctx, skip, take)

}

