package publisher

import "context"

type Publisher interface {
	Send(ctx context.Context, message string) error
}
