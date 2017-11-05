-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE esi_keys (
    char_id BIGINT NOT NULL,
    purpose VARCHAR(8) NOT NULL,
    access_token TEXT,
    token_type TEXT,
    refresh_token TEXT,
    expiry timestamp(0) with time zone NOT NULL,
    PRIMARY KEY ( char_id, purpose )
);

CREATE TABLE accounts (
    account_id SERIAL,
    main_char_id BIGINT,
    PRIMARY KEY ( account_id )
);

CREATE TABLE account_chars (
    account_id INTEGER NOT NULL,
    char_id BIGINT NOT NULL,
    PRIMARY KEY ( account_id, char_id )
);

CREATE TABLE chars (
    char_id BIGINT NOT NULL,
    is_jf_pilot BOOLEAN NOT NULL,
    is_contractor BOOLEAN NOT NULL,
    PRIMARY KEY ( char_id )
);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS esi_keys, accounts, account_chars, chars;