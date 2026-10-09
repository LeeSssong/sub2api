package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyToolMappingRepoStub struct {
	GroupRepository
	requested []int64
	err       error
}

func (s *apiKeyToolMappingRepoStub) ReadGroupToolMappings(_ context.Context, ids []int64) (map[int64]GroupToolMapping, error) {
	s.requested = append([]int64(nil), ids...)
	return map[int64]GroupToolMapping{2: {GroupID: 2, ToolIDs: []string{"claude"}}}, s.err
}
func (*apiKeyToolMappingRepoStub) ReplaceGroupToolMapping(context.Context, int64, []string, int64) (GroupToolMapping, error) {
	panic("read only")
}

func TestAPIKeyGetGroupToolMappingsUsesNativeRepository(t *testing.T) {
	repo := &apiKeyToolMappingRepoStub{}
	s := &APIKeyService{groupRepo: repo}
	mappings, err := s.GetGroupToolMappings(context.Background(), []int64{2, 3})
	require.NoError(t, err)
	require.Equal(t, []int64{2, 3}, repo.requested)
	require.Equal(t, []string{"claude"}, mappings[2].ToolIDs)
	repo.err = errors.New("lookup failed")
	_, err = s.GetGroupToolMappings(context.Background(), []int64{2})
	require.ErrorContains(t, err, "lookup failed")
	// Older narrow test repositories without this optional capability stay compatible.
	mappings, err = (&APIKeyService{}).GetGroupToolMappings(context.Background(), []int64{2})
	require.NoError(t, err)
	require.Empty(t, mappings)
}
