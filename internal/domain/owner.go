package domain

import "github.com/google/uuid"

type Users struct {
	ID           uuid.UUID `db:"id"`
	UserName     string    `db:"user_name"`
	PasswordHash *string   `db:"password_hash"`
	StudentCode  string    `db:"student_code"`
	Email        *string   `db:"email"`
	AuthProvider string    `db:"auth_provider"`
	GoogleID     *string   `db:"google_id"`
}

type UsersTable struct {
	ID           string
	UserName     string
	PasswordHash string
	StudentCode  string
	Email        string
	AuthProvider string
	GoogleID     string
}

func GetUserTable() UsersTable {
	return UsersTable{
		ID:           "id",
		UserName:     "user_name",
		PasswordHash: "password_hash",
		StudentCode:  "student_code",
		Email:        "email",
		AuthProvider: "auth_provider",
		GoogleID:     "google_id",
	}
}

func (t UsersTable) GetTableName() string {
	return "users"
}
