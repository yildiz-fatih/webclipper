package models

import (
	"database/sql"
	"errors"
	"time"
)

type Clip struct {
	ID        string
	URL       string
	CleanHTML string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type ClipModel struct {
	DB *sql.DB
}

var ErrNotFound = errors.New("record not found")

func (m *ClipModel) Get(id string) (Clip, error) {
	query := `SELECT id, url, clean_html, created_at, expires_at 
	FROM clips 
	WHERE expires_at > NOW() AND id = $1`

	var c Clip

	err := m.DB.QueryRow(query, id).Scan(&c.ID, &c.URL, &c.CleanHTML, &c.CreatedAt, &c.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Clip{}, ErrNotFound
		}
		return Clip{}, err
	}

	return c, nil
}

func (m *ClipModel) Insert(url, cleanHTML string) (Clip, error) {
	query := `INSERT INTO clips (url, clean_html) 
	VALUES ($1, $2)
	RETURNING id, url, clean_html, created_at, expires_at`

	var clip Clip
	err := m.DB.QueryRow(query, url, cleanHTML).Scan(
		&clip.ID,
		&clip.URL,
		&clip.CleanHTML,
		&clip.CreatedAt,
		&clip.ExpiresAt,
	)
	if err != nil {
		return Clip{}, err
	}

	return clip, nil
}
