package models

type Account struct {
	ID     uint64  `json:"id"`
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Orders []Order `json:"orders"`
}
