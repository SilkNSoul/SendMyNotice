package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Lead struct {
	ID        int
	Email     string
	Name      string
	CreatedAt time.Time
	Paid      bool
	Reminder1 bool // Sent 1 hour after
	Reminder2 bool // Sent 5 days after
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
	
	// Create Table if not exists
	query := `
	CREATE TABLE IF NOT EXISTS leads (
		id SERIAL PRIMARY KEY,
		email TEXT NOT NULL,
		name TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		paid BOOLEAN DEFAULT FALSE,
		reminder_1_sent BOOLEAN DEFAULT FALSE,
		reminder_2_sent BOOLEAN DEFAULT FALSE
	);`
	
	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return &DB{sql: db}, nil
}

func (d *DB) CreateLead(email, name string) error {
	// Upsert: If email exists, update name, otherwise insert
	// Note: For MVP, simple insert is fine, or check existence
	_, err := d.sql.Exec("INSERT INTO leads (email, name) VALUES ($1, $2)", email, name)
	return err
}

func (d *DB) MarkPaid(email string) error {
	_, err := d.sql.Exec("UPDATE leads SET paid = TRUE WHERE email = $1", email)
	return err
}

func (d *DB) GetPendingReminders() ([]Lead, error) {
	// Find people who haven't paid, created > 1 hour ago, and haven't got reminder 1
	rows, err := d.sql.Query(`
		SELECT id, email, name, created_at 
		FROM leads 
		WHERE paid = FALSE 
		AND reminder_1_sent = FALSE 
		AND created_at < NOW() - INTERVAL '1 hour'
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		if err := rows.Scan(&l.ID, &l.Email, &l.Name, &l.CreatedAt); err == nil {
			leads = append(leads, l)
		}
	}
	return leads, nil
}

func (d *DB) MarkReminderSent(id int, reminderNum int) error {
	col := "reminder_1_sent"
	if reminderNum == 2 { col = "reminder_2_sent" }
	query := fmt.Sprintf("UPDATE leads SET %s = TRUE WHERE id = $1", col)
	_, err := d.sql.Exec(query, id)
	return err
}