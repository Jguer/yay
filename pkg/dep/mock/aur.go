package mock

import (
	"context"

	"github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
)

type GetFunc func(ctx context.Context, query *metadata.AURQuery) ([]*aur.Pkg, error)

type MockAUR struct {
	GetFn GetFunc
}

func (m *MockAUR) Get(ctx context.Context, query *metadata.AURQuery) ([]*aur.Pkg, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, query)
	}

	panic("implement me")
}
