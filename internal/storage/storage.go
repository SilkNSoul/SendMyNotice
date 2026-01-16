package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Lead struct {
	ID          int
	Email       string
	Name        string
	CreatedAt   time.Time
	Paid        bool
	EmailStep   int       // Which email # did they last receive?
	LastEmailAt time.Time // When did we send it?
}

type DB struct {
	sql *sql.DB
}

func NewPostgres(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database connection failed: %w", err)
	}
	
	// UPDATED SCHEMA: handling the drip campaign state
	query := `
	CREATE TABLE IF NOT EXISTS leads (
		id SERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		name TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		paid BOOLEAN DEFAULT FALSE,
		email_step INTEGER DEFAULT 0,
		last_email_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	
	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return &DB{sql: db}, nil
}

// UpsertLead: Safe to call multiple times for the same email
func (d *DB) UpsertLead(email, name string) error {
	query := `
		INSERT INTO leads (email, name, last_email_at) 
		VALUES ($1, $2, NOW())
		ON CONFLICT (email) DO UPDATE 
		SET name = EXCLUDED.name;`
	_, err := d.sql.Exec(query, email, name)
	return err
}

func (d *DB) MarkPaid(email string) error {
	_, err := d.sql.Exec("UPDATE leads SET paid = TRUE WHERE email = $1", email)
	return err
}

// GetStaleLeads finds people who haven't paid and are ready for the next email
func (d *DB) GetStaleLeads(delay time.Duration) ([]Lead, error) {
	// "Give me leads who haven't paid, where the time since the last email is > delay"
	rows, err := d.sql.Query(`
		SELECT id, email, name, created_at, email_step, last_email_at
		FROM leads 
		WHERE paid = FALSE 
		AND last_email_at < NOW() - $1::INTERVAL
		LIMIT 50
	`, fmt.Sprintf("%d seconds", int(delay.Seconds())))
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		// Handle Nullable timestamps if needed, but defaults handle it
		if err := rows.Scan(&l.ID, &l.Email, &l.Name, &l.CreatedAt, &l.EmailStep, &l.LastEmailAt); err == nil {
			leads = append(leads, l)
		}
	}
	return leads, nil
}

func (d *DB) IncrementEmailStep(id int, newStep int) error {
	_, err := d.sql.Exec("UPDATE leads SET email_step = $1, last_email_at = NOW() WHERE id = $2", newStep, id)
	return err
}

func (d *DB) CreateLead(email, name string) error {
	// Upsert: If email exists, update name, otherwise insert
	// Note: For MVP, simple insert is fine, or check existence
	_, err := d.sql.Exec("INSERT INTO leads (email, name) VALUES ($1, $2)", email, name)
	return err
}