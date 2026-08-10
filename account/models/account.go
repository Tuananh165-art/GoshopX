package models

import "time"

type Account struct {
	ID                     uint64     `gorm:"primaryKey;autoIncrement"`
	Name                   string     `json:"name"`
	Email                  string     `json:"email"`
	AvatarURL              string     `json:"avatar_url"`
	GoogleAvatarURL        string     `json:"-"`
	Phone                  string     `json:"phone"`
	ShippingAddress        string     `json:"shipping_address"`
	PasswordResetOTPHash   string     `json:"-"`
	PasswordResetExpiresAt *time.Time `json:"-"`
	PasswordResetAttempts  int        `json:"-"`
	Password               string     `json:"password"`
	GoogleSubject          string     `json:"-" gorm:"index;size:255"`
	RoleID                 int        `json:"role_id" gorm:"not null;default:0;index"`
	// Role is a compatibility value for JWT/GraphQL and is not persisted.
	Role   string `json:"-" gorm:"-"`
	Status string `json:"status" gorm:"not null;default:active"`
}

const (
	RoleCustomerID = 0
	RoleAdminID    = 1

	RoleCustomer        = "customer"
	RoleSeller          = "seller"
	RoleSupportAdmin    = "support_admin"
	RoleOperationsAdmin = "operations_admin"
	RolePlatformAdmin   = "platform_admin"

	StatusActive    = "active"
	StatusSuspended = "suspended"
)

func RoleName(roleID int) string {
	if roleID == RoleAdminID {
		return "admin"
	}
	return RoleCustomer
}

func RoleIDForName(role string) int {
	if role != "" && role != RoleCustomer {
		return RoleAdminID
	}
	return RoleCustomerID
}

func (account *Account) NormalizeRole() {
	if account.RoleID != RoleAdminID {
		account.RoleID = RoleCustomerID
	}
	account.Role = RoleName(account.RoleID)
}

type AdminAuditEvent struct {
	EventID         string    `gorm:"primaryKey"`
	ActorAccountID  uint64    `gorm:"index;not null"`
	TargetAccountID uint64    `gorm:"index;not null"`
	Action          string    `gorm:"not null"`
	Outcome         string    `gorm:"not null"`
	RequestID       string    `gorm:"index"`
	OccurredAt      time.Time `gorm:"not null"`
}
