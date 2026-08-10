package internal

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/Tuananh165art/GoshopX/account/config"
	"github.com/Tuananh165art/GoshopX/account/models"
	"github.com/Tuananh165art/GoshopX/pkg/auth"
	"github.com/Tuananh165art/GoshopX/pkg/crypt"
	"github.com/Tuananh165art/GoshopX/pkg/events"
	"github.com/Tuananh165art/GoshopX/pkg/kafka"
	"google.golang.org/api/idtoken"
)

type Service interface {
	Register(ctx context.Context, name, email, password string) (string, error)
	Login(ctx context.Context, email, password string) (string, error)
	LoginWithGoogle(ctx context.Context, credential string) (string, error)
	GetAccount(ctx context.Context, id uint64) (*models.Account, error)
	UpdateProfile(ctx context.Context, id uint64, name, email, avatarURL, phone, shippingAddress string) (*models.Account, error)
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, email, otp, newPassword string) error
	GetAccounts(ctx context.Context, skip uint64, take uint64) ([]*models.Account, error)
	SetAccountStatus(ctx context.Context, actorID, targetID uint64, actorRole, status, requestID string) (*models.Account, error)
	SetAccountRole(ctx context.Context, actorID, targetID uint64, actorRole, role, requestID string) (*models.Account, error)
	GetProducer() sarama.AsyncProducer
}

type accountService struct {
	repository Repository
	producer   sarama.AsyncProducer
}

func NewService(r Repository, producers ...sarama.AsyncProducer) Service {
	var producer sarama.AsyncProducer
	if len(producers) > 0 {
		producer = producers[0]
	}
	return &accountService{repository: r, producer: producer}
}

func (service accountService) GetProducer() sarama.AsyncProducer { return service.producer }

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
		RoleID:   models.RoleCustomerID,
		Role:     models.RoleCustomer,
		Status:   models.StatusActive,
	}
	account, err := service.repository.PutAccount(ctx, acc)
	if err != nil {
		return "", err
	}
	token, err := auth.GenerateToken(account.ID, account.Role)
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
	if account.Status == "" {
		account.Status = models.StatusActive
	}
	account.NormalizeRole()
	if account.Status != models.StatusActive {
		return "", errors.New("account suspended")
	}

	token, err := auth.GenerateToken(account.ID, account.Role)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (service accountService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	account, err := service.repository.GetAccountByEmail(ctx, email)
	if err != nil {
		return nil
	}
	if account.PasswordResetOTPHash != "" && account.PasswordResetExpiresAt != nil && time.Now().UTC().Before(*account.PasswordResetExpiresAt) {
		return nil
	}
	number, err := cryptorand.Int(cryptorand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	otp := fmt.Sprintf("%06d", number.Int64())
	hash, err := crypt.HashPassword(otp)
	if err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	account.PasswordResetOTPHash = hash
	account.PasswordResetExpiresAt = &expiresAt
	account.PasswordResetAttempts = 0
	if err := service.repository.SavePasswordReset(ctx, account); err != nil {
		return err
	}
	event := events.NewForRecipient("auth.password_reset_otp", account.ID, account.Email, "password-reset-"+strconv.FormatUint(account.ID, 10), map[string]any{"otp": otp})
	return kafka.SendMessage(service, event, config.AccountEventsTopic)
}

func (service accountService) ResetPassword(ctx context.Context, email, otp, newPassword string) error {
	if len(newPassword) < 8 || strings.TrimSpace(otp) == "" {
		return errors.New("invalid password reset request")
	}
	account, err := service.repository.GetAccountByEmail(ctx, strings.TrimSpace(email))
	if err != nil || account.PasswordResetExpiresAt == nil || time.Now().UTC().After(*account.PasswordResetExpiresAt) {
		return errors.New("invalid or expired otp")
	}
	if account.PasswordResetAttempts >= 5 || crypt.VerifyPassword(otp, account.PasswordResetOTPHash) != nil {
		account.PasswordResetAttempts++
		_ = service.repository.SavePasswordReset(ctx, account)
		return errors.New("invalid or expired otp")
	}
	hash, err := crypt.HashPassword(newPassword)
	if err != nil {
		return err
	}
	account.Password = hash
	return service.repository.UpdatePassword(ctx, account)
}

func (service accountService) LoginWithGoogle(ctx context.Context, credential string) (string, error) {
	if config.GoogleClientID == "" {
		return "", errors.New("google login is not configured")
	}
	payload, err := idtoken.Validate(ctx, credential, config.GoogleClientID)
	if err != nil || payload == nil {
		return "", errors.New("invalid google identity")
	}
	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)
	avatarURL, _ := payload.Claims["picture"].(string)
	if email == "" || !emailVerified {
		return "", errors.New("invalid google identity")
	}

	account, err := service.repository.GetAccountByEmail(ctx, email)
	if err != nil {
		account, err = service.repository.PutAccount(ctx, models.Account{
			Name:            name,
			Email:           email,
			AvatarURL:       avatarURL,
			GoogleAvatarURL: avatarURL,
			GoogleSubject:   payload.Subject,
			RoleID:          models.RoleCustomerID,
			Role:            models.RoleCustomer,
			Status:          models.StatusActive,
		})
		if err != nil {
			return "", err
		}
	} else {
		if account.Status != "" && account.Status != models.StatusActive {
			return "", errors.New("account suspended")
		}
		if account.GoogleSubject != payload.Subject || account.Name == "" || (account.AvatarURL == "" && avatarURL != "") {
			account.GoogleSubject = payload.Subject
			if account.Name == "" {
				account.Name = name
			}
			if account.AvatarURL == "" {
				account.AvatarURL = avatarURL
			}
			account.GoogleAvatarURL = avatarURL
			if err := service.repository.UpdateGoogleIdentity(ctx, account); err != nil {
				return "", err
			}
		}
	}
	return auth.GenerateToken(account.ID, account.Role)
}

func (service accountService) SetAccountStatus(ctx context.Context, actorID, targetID uint64, actorRole, status, requestID string) (*models.Account, error) {
	if !canManageAccounts(actorRole) || actorID == targetID || (status != models.StatusActive && status != models.StatusSuspended) {
		return nil, errors.New("forbidden account status change")
	}
	target, err := service.repository.GetAccountByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	target.Status = status
	if err := service.repository.UpdateAccount(ctx, target); err != nil {
		return nil, err
	}
	if err := service.audit(ctx, actorID, targetID, "account_status_changed", "success", requestID); err != nil {
		return target, err
	}
	return target, service.publishAudit(actorID, targetID, actorRole, "account_status_changed", "success", requestID)
}

func (service accountService) SetAccountRole(ctx context.Context, actorID, targetID uint64, actorRole, role, requestID string) (*models.Account, error) {
	if (actorRole != "admin" && actorRole != models.RolePlatformAdmin) || actorID == targetID || !validRole(role) {
		return nil, errors.New("forbidden account role change")
	}
	target, err := service.repository.GetAccountByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	target.Role = role
	target.RoleID = models.RoleIDForName(role)
	if err := service.repository.UpdateAccount(ctx, target); err != nil {
		return nil, err
	}
	if err := service.audit(ctx, actorID, targetID, "account_role_changed", "success", requestID); err != nil {
		return target, err
	}
	return target, service.publishAudit(actorID, targetID, actorRole, "account_role_changed", "success", requestID)
}

func (service accountService) audit(ctx context.Context, actorID, targetID uint64, action, outcome, requestID string) error {
	return service.repository.SaveAdminAuditEvent(ctx, models.AdminAuditEvent{
		EventID: fmt.Sprintf("%d-%d-%d", actorID, targetID, time.Now().UnixNano()), ActorAccountID: actorID,
		TargetAccountID: targetID, Action: action, Outcome: outcome, RequestID: requestID, OccurredAt: time.Now().UTC(),
	})
}

func canManageAccounts(role string) bool {
	return role == "admin" || role == models.RoleSupportAdmin || role == models.RoleOperationsAdmin || role == models.RolePlatformAdmin
}

func validRole(role string) bool {
	return role == models.RoleCustomer || role == "admin" || role == models.RoleSeller || role == models.RoleSupportAdmin || role == models.RoleOperationsAdmin || role == models.RolePlatformAdmin
}

func (service accountService) publishAudit(actorID, targetID uint64, actorRole, action, outcome, requestID string) error {
	return kafka.SendMessage(service, events.New(action, actorID, fmt.Sprintf("%d", targetID), map[string]any{"target_account_id": targetID, "actor_role": actorRole, "outcome": outcome, "request_id": requestID}), config.AdminEventsTopic)
}

func (service accountService) GetAccount(ctx context.Context, id uint64) (*models.Account, error) {
	return service.repository.GetAccountByID(ctx, id)
}

func (service accountService) UpdateProfile(ctx context.Context, id uint64, name, email, avatarURL, phone, shippingAddress string) (*models.Account, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" || !strings.Contains(email, "@") {
		return nil, errors.New("name and valid email are required")
	}
	account, err := service.repository.GetAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if email != account.Email {
		other, lookupErr := service.repository.GetAccountByEmail(ctx, email)
		if lookupErr == nil && other.ID != id {
			return nil, errors.New("email already in use")
		}
	}
	account.Name = name
	account.Email = email
	account.GoogleAvatarURL = strings.TrimSpace(account.GoogleAvatarURL)
	account.AvatarURL = strings.TrimSpace(avatarURL)
	if strings.HasPrefix(account.AvatarURL, "data:") && (!strings.HasPrefix(account.AvatarURL, "data:image/") || len(account.AvatarURL) > 3500000) {
		return nil, errors.New("avatar must be an image smaller than 2 MB")
	}
	if account.AvatarURL == "" {
		account.AvatarURL = account.GoogleAvatarURL
	}
	account.Phone = strings.TrimSpace(phone)
	account.ShippingAddress = strings.TrimSpace(shippingAddress)
	if err := service.repository.UpdateAccount(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (service accountService) GetAccounts(ctx context.Context, skip uint64, take uint64) ([]*models.Account, error) {
	if take > 100 || (skip == 0 && take == 0) {
		take = 100
	}

	return service.repository.ListAccounts(ctx, skip, take)

}
