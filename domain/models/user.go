package models

import "time"

type Role string

const (
	RoleEmployee Role = "employee"
	RoleAdmin    Role = "admin"
)

type User struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Role      Role      `json:"role" db:"role"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
