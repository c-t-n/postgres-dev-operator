package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PostgresDriverConnector struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
}

func createConnector(pq *PostgresDriverConnector) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		pq.Username,
		pq.Password,
		pq.Host,
		pq.Port,
		pq.Database,
	)
	db, err := sql.Open("postgress", dsn)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (pq *PostgresDriverConnector) CreateDatabase(DatabaseName string, username string, password string) error {
	conn, err := createConnector(pq)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Check if user exists or not
	rows, err := conn.Query("SELECT 1 FROM pg_roles WHERE rolname=?", username)
	if err != nil {
		return err
	}

	if !rows.Next() {
		// If the user doesn't exists, create it with passord
		_, err = conn.Query(
			"CREATE USER ? WITH PASSWORD ?;",
			username,
			password,
		)
		if err != nil {
			return err
		}
	}

	_, err = conn.Query(
		"CREATE DATABASE ? WITH OWNER ?;",
		DatabaseName,
		username,
	)
	if err != nil {
		return err
	}

	return nil
}

func (pq *PostgresDriverConnector) DeleteDatabase(DatabaseName string) error {
	conn, err := createConnector(pq)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Query(
		"DROP DATABASE ?;",
		DatabaseName,
	)
	if err != nil {
		return err
	}

	return nil
}
