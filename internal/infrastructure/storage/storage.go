package storage

import (
	"context"
	"fmt"

	"vibechat/internal/domain"
	"vibechat/internal/infrastructure/storage/mysql"
	"vibechat/internal/infrastructure/storage/postgres"
)

type Storage struct {
	Users    domain.UserRepository
	Chats    domain.ChatRepository
	Messages domain.MessageRepository
	closer   func()
	pinger   func(ctx context.Context) error
}

func (s *Storage) Close() { s.closer() }

func (s *Storage) Ping(ctx context.Context) error {
	if s.pinger != nil {
		return s.pinger(ctx)
	}
	return nil
}

func New(ctx context.Context, cfg Config) (*Storage, error) {
	switch cfg.Type {
	case TypePostgres:
		if cfg.Postgres == nil {
			return nil, fmt.Errorf("storage: postgres config required for type %q", cfg.Type)
		}
		pool, err := postgres.NewPool(ctx, *cfg.Postgres)
		if err != nil {
			return nil, err
		}
		return &Storage{
			Users:    postgres.NewUserRepository(pool),
			Chats:    postgres.NewChatRepository(pool),
			Messages: postgres.NewMessageRepository(pool),
			closer:   func() { pool.Close() },
			pinger:   pool.Ping,
		}, nil

	case TypeMySQL:
		if cfg.MySQL == nil {
			return nil, fmt.Errorf("storage: mysql config required for type %q", cfg.Type)
		}
		db, err := mysql.NewDB(ctx, *cfg.MySQL)
		if err != nil {
			return nil, err
		}
		return &Storage{
			Users:    mysql.NewUserRepository(db),
			Chats:    mysql.NewChatRepository(db),
			Messages: mysql.NewMessageRepository(db),
			closer:   func() { _ = db.Close() },
			pinger:   db.PingContext,
		}, nil

	default:
		return nil, fmt.Errorf("storage: unknown type %q", cfg.Type)
	}
}
