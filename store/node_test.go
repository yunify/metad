package store

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRelativePath(t *testing.T) {

	s := newStore()
	root := s.Root
	ctx := context.Background()
	s.Put(ctx, "/1/2/3", "v")
	n1 := s.internalGet(ctx, "/1")
	n2 := s.internalGet(ctx, "/1/2")
	n3 := s.internalGet(ctx, "/1/2/3")

	assert.Equal(t, "/1/2/3", n3.RelativePath(root))
	assert.Equal(t, "/2/3", n3.RelativePath(n1))
	assert.Equal(t, "/3", n3.RelativePath(n2))
	assert.Equal(t, "/", n3.RelativePath(n3))

}
