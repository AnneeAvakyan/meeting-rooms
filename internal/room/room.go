package room

import "time"

type Room struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Capacity  int       `json:"capacity" db:"capacity"`
	Equipment string    `json:"equipment" db:"equipment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
