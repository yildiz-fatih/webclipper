package models

import (
	"database/sql"
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
