package subscriber

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("subscriber not found")

type Repository interface {
	Create(ctx context.Context, sub Subscriber) (Subscriber, error)
	List(ctx context.Context) ([]Subscriber, error)
	GetByID(ctx context.Context, id string) (Subscriber, error)
	Deactivate(ctx context.Context, id string) (Subscriber, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, sub Subscriber) (Subscriber, error) {
	const query = `
		INSERT INTO webhook_subscribers (
			id, name, url, event_type, secret, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, url, event_type, secret, status, created_at, updated_at`

	created, err := scanSubscriber(r.db.QueryRowContext(
		ctx,
		query,
		sub.ID,
		sub.Name,
		sub.URL,
		sub.EventType,
		sub.Secret,
		sub.Status,
		sub.CreatedAt,
		sub.UpdatedAt,
	))
	if err != nil {
		return Subscriber{}, fmt.Errorf("create subscriber: %w", err)
	}

	return created, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Subscriber, error) {
	const query = `
		SELECT id, name, url, event_type, secret, status, created_at, updated_at
		FROM webhook_subscribers
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list subscribers: %w", err)
	}
	defer rows.Close()

	subs := make([]Subscriber, 0)
	for rows.Next() {
		sub, err := scanSubscriber(rows)
		if err != nil {
			return nil, fmt.Errorf("scan subscriber: %w", err)
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list subscribers rows: %w", err)
	}

	return subs, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Subscriber, error) {
	const query = `
		SELECT id, name, url, event_type, secret, status, created_at, updated_at
		FROM webhook_subscribers
		WHERE id = $1`

	sub, err := scanSubscriber(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Subscriber{}, ErrNotFound
	}
	if err != nil {
		return Subscriber{}, fmt.Errorf("get subscriber: %w", err)
	}

	return sub, nil
}

func (r *PostgresRepository) Deactivate(ctx context.Context, id string) (Subscriber, error) {
	const query = `
		UPDATE webhook_subscribers
		SET status = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, url, event_type, secret, status, created_at, updated_at`

	sub, err := scanSubscriber(r.db.QueryRowContext(ctx, query, id, StatusInactive))
	if errors.Is(err, sql.ErrNoRows) {
		return Subscriber{}, ErrNotFound
	}
	if err != nil {
		return Subscriber{}, fmt.Errorf("deactivate subscriber: %w", err)
	}

	return sub, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSubscriber(row scanner) (Subscriber, error) {
	var sub Subscriber
	err := row.Scan(
		&sub.ID,
		&sub.Name,
		&sub.URL,
		&sub.EventType,
		&sub.Secret,
		&sub.Status,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)
	if err != nil {
		return Subscriber{}, err
	}
	return sub, nil
}
