package challenge

import "context"

type Repository interface {
	Create(ctx context.Context, challenge *AuthChallenge) error
	Get(ctx context.Context, id string) (*AuthChallenge, error)
	Consume(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}
