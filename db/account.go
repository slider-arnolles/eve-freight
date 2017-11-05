package db

import "github.com/jmoiron/sqlx"

// Account is a user's account with the service
type Account struct {
	ID              int   `db:"account_id"`
	MainCharacterID int64 `db:"main_char_id"`
}

// GetOrCreateAccount looks up the account given a user id
func GetOrCreateAccount(db *sqlx.DB, char int64) *Account {
	tx := db.MustBegin()
	defer tx.Commit()

	acc := &Account{}

	tx.Get(acc, "SELECT accounts.account_id AS account_id, accounts.main_char_id AS main_char_id FROM account_chars INNER JOIN accounts ON account_chars.account_id = accounts.account_id")
}
