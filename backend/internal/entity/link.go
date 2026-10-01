package entity

import (
	"errors"
	"time"
)

type Link struct {
	ID          string
	Code        string
	OriginalURL string
	Clicks      int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type LinkUpdate struct {
	OriginalURL *string
	Code        *string
}

type LinkPage struct {
	Items  []Link
	Total  int
	Limit  int
	Offset int
}

var (
	ErrLinkNotFound      = errors.New("link not found")
	ErrCodeAlreadyExists = errors.New("code already exists")
)
