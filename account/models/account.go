package models

type Account struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
