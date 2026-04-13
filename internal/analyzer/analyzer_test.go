package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNodeID_Deterministic(t *testing.T) {
	id1 := NewNodeID("pkg/foo", "Bar", Function)
	id2 := NewNodeID("pkg/foo", "Bar", Function)
	assert.Equal(t, id1, id2)
}

func TestNewNodeID_Distinct(t *testing.T) {
	idFunc := NewNodeID("pkg/foo", "Bar", Function)
	idType := NewNodeID("pkg/foo", "Bar", Type)
	assert.NotEqual(t, idFunc, idType)
}

func TestNodeKind_String(t *testing.T) {
	tests := []struct {
		kind NodeKind
		want string
	}{
		{Function, "Function"},
		{Method, "Method"},
		{Type, "Type"},
		{Interface, "Interface"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}

func TestEdgeKind_String(t *testing.T) {
	tests := []struct {
		kind EdgeKind
		want string
	}{
		{Calls, "Calls"},
		{Implements, "Implements"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}
