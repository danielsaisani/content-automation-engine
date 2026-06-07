package api

import (
	"context"
)

type Port int

type Application interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Port(ctx context.Context) Port
}
