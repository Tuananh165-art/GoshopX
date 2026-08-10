package internal

import (
	"context"
	"log"

	"github.com/Tuananh165art/GoshopX/account/models"
	"github.com/Tuananh165art/GoshopX/pkg/migrations"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
)

type Repository interface {
	Close()
	PutAccount(ctx context.Context, a models.Account) (*models.Account, error)
	GetAccountByEmail(ctx context.Context, email string) (*models.Account, error)
	GetAccountByID(ctx context.Context, id uint64) (*models.Account, error)
	ListAccounts(ctx context.Context, skip uint64, take uint64) ([]*models.Account, error)
	UpdateAccount(ctx context.Context, account *models.Account) error
	UpdateGoogleIdentity(ctx context.Context, account *models.Account) error
	SavePasswordReset(ctx context.Context, account *models.Account) error
	UpdatePassword(ctx context.Context, account *models.Account) error
	SaveAdminAuditEvent(ctx context.Context, event models.AdminAuditEvent) error
}

type postgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) (Repository, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	err = sqlDB.Ping()
	if err != nil {
		return nil, err
	}

	err = migrations.Run(db, "account", "001_initial", func(db *gorm.DB) error {
		return db.AutoMigrate(&models.Account{}, &models.AdminAuditEvent{})
	})
	if err == nil {
		err = migrations.Run(db, "account", "002_role_id", func(db *gorm.DB) error {
			if !db.Migrator().HasColumn(&models.Account{}, "role") {
				return nil
			}
			if err := db.Exec("UPDATE accounts SET role_id = CASE WHEN role IN (?, ?, ?, ?) THEN 1 ELSE 0 END", models.RoleSeller, models.RoleSupportAdmin, models.RoleOperationsAdmin, models.RolePlatformAdmin).Error; err != nil {
				return err
			}
			return db.Migrator().DropColumn(&models.Account{}, "role")
		})
	}
	if err != nil {
		log.Println("Error during migrations:", err)
	}

	return &postgresRepository{db}, nil
}

func (repository *postgresRepository) Close() {
	sqlDB, err := repository.db.DB()
	if err == nil {
		err = sqlDB.Close()
		if err != nil {
			log.Println("Error closing postgres repository")
			log.Println(err)
		}
	}
}

func (repository *postgresRepository) Ping() error {
	sqlDB, err := repository.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (repository *postgresRepository) PutAccount(ctx context.Context, a models.Account) (*models.Account, error) {
	if a.Role != "" {
		a.RoleID = models.RoleIDForName(a.Role)
	}
	a.NormalizeRole()
	if a.Status == "" {
		a.Status = models.StatusActive
	}
	if err := repository.db.WithContext(ctx).Create(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func normalizeAccount(account *models.Account) *models.Account {
	account.NormalizeRole()
	return account
}

func (repository *postgresRepository) SaveAdminAuditEvent(ctx context.Context, event models.AdminAuditEvent) error {
	return repository.db.WithContext(ctx).Create(&event).Error
}

func (repository *postgresRepository) GetAccountByEmail(ctx context.Context, email string) (*models.Account, error) {
	var account models.Account
	if err := repository.db.WithContext(ctx).First(&account, "email = ?", email).Error; err != nil {
		return nil, err
	}
	return normalizeAccount(&account), nil
}

func (repository *postgresRepository) GetAccountByID(ctx context.Context, id uint64) (*models.Account, error) {
	var account models.Account
	if err := repository.db.WithContext(ctx).First(&account, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return normalizeAccount(&account), nil
}

func (repository *postgresRepository) UpdateAccount(ctx context.Context, account *models.Account) error {
	if account.Role != "" {
		account.RoleID = models.RoleIDForName(account.Role)
	}
	account.NormalizeRole()
	return repository.db.WithContext(ctx).Model(&models.Account{}).Where("id = ?", account.ID).Updates(map[string]interface{}{
		"name": account.Name, "email": account.Email, "avatar_url": account.AvatarURL, "google_avatar_url": account.GoogleAvatarURL,
		"phone": account.Phone, "shipping_address": account.ShippingAddress,
		"role_id": account.RoleID, "status": account.Status,
	}).Error
}

func (repository *postgresRepository) UpdateGoogleIdentity(ctx context.Context, account *models.Account) error {
	return repository.db.WithContext(ctx).Model(&models.Account{}).Where("id = ?", account.ID).
		Updates(map[string]interface{}{"name": account.Name, "avatar_url": account.AvatarURL, "google_avatar_url": account.GoogleAvatarURL, "google_subject": account.GoogleSubject}).Error
}

func (repository *postgresRepository) SavePasswordReset(ctx context.Context, account *models.Account) error {
	return repository.db.WithContext(ctx).Model(&models.Account{}).Where("id = ?", account.ID).Updates(map[string]interface{}{"password_reset_otp_hash": account.PasswordResetOTPHash, "password_reset_expires_at": account.PasswordResetExpiresAt, "password_reset_attempts": account.PasswordResetAttempts}).Error
}

func (repository *postgresRepository) UpdatePassword(ctx context.Context, account *models.Account) error {
	return repository.db.WithContext(ctx).Model(&models.Account{}).Where("id = ?", account.ID).Updates(map[string]interface{}{"password": account.Password, "password_reset_otp_hash": nil, "password_reset_expires_at": nil, "password_reset_attempts": 0}).Error
}

func (repository *postgresRepository) ListAccounts(ctx context.Context, skip uint64, take uint64) ([]*models.Account, error) {
	var accounts []*models.Account
	if err := repository.db.WithContext(ctx).Offset(int(skip)).Limit(int(take)).Find(&accounts).Error; err != nil {
		return nil, err
	}
	for _, account := range accounts {
		normalizeAccount(account)
	}
	return accounts, nil
}
