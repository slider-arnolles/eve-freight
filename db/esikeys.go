package db

import "time"

// ESIKeys holds the auth keys for a character
type ESIKeys struct {
	CharacterID  int64     `db:"char_id"`
	Purpose      string    `db:"purpose"`
	AccessToken  string    `db:"access_token"`
	TokenType    string    `db:"token_type"`
	RefreshToken string    `db:"refresh_token"`
	Expiry       time.Time `db:"expiry"`
}
