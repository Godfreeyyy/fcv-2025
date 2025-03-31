package userrepository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/ports/secondary"
	"gitlab.com/fcv-2025.net/internal/domain"
	querybuilder "gitlab.com/fcv-2025.net/internal/utils"
)

var _ secondary.UserPort = &userRepo{}

type userRepo struct {
	db     *sqlx.DB
	logger primary.Logger
	schema string
}

func New(db *sqlx.DB, logger primary.Logger, schema string) secondary.UserPort {
	return &userRepo{
		db:     db,
		logger: logger,
		schema: os.Getenv("DB_SCHEMA"),
	}
}

func (u userRepo) Create(ctx context.Context, user *domain.Users) error {
	userTbl := domain.GetUserTable()
	query, args := querybuilder.NewQueryBuilder(u.schema).Insert(
		userTbl.UserName, userTbl.Email, userTbl.PasswordHash,
		userTbl.StudentCode,
		userTbl.AuthProvider, userTbl.GoogleID,
	).
		Into(userTbl.GetTableName()).
		Values(
			user.UserName, user.Email, user.PasswordHash,
			user.StudentCode,
			user.AuthProvider, user.GoogleID,
		).
		Build()

	query = sqlx.Rebind(sqlx.DOLLAR, query)
	_, err := u.db.ExecContext(ctx, query, args...)

	return err
}

func (u userRepo) Save(user *domain.Users) error {
	//TODO implement me
	panic("implement me")
}

func (u userRepo) Delete(id string) error {
	//TODO implement me
	panic("implement me")
}

func (u userRepo) Get(id string) (*domain.Users, error) {
	//TODO implement me
	panic("implement me")
}

func (u userRepo) GetByEmail(email string) (*domain.Users, error) {
	//TODO implement me
	panic("implement me")
}

func (u userRepo) GetByUserName(userName string) (*domain.Users, error) {
	panic("implement me")
}

func (u userRepo) VerifyUserNAmePassword(userName, password string) (*domain.Users, error) {
	panic("implement me")
}

func (u userRepo) GetByGoogleID(ctx context.Context, googleID string) (*domain.Users, error) {
	userTbl := domain.GetUserTable()
	query, args := querybuilder.NewQueryBuilder(u.schema).
		Select(
			userTbl.ID,
			userTbl.UserName, userTbl.Email, userTbl.PasswordHash,
			userTbl.StudentCode, userTbl.Email,
			userTbl.AuthProvider, userTbl.GoogleID,
		).
		From(userTbl.GetTableName()).
		Where(fmt.Sprintf("%s = ?", userTbl.GoogleID), googleID).
		Build()

	query = sqlx.Rebind(sqlx.DOLLAR, query)
	var user domain.Users
	err := u.db.GetContext(ctx, &user, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}
