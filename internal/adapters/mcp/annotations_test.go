package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Las dos tools son de SÓLO LECTURA: deben anunciarse como no destructivas para
// que el cliente MCP no las marque como [destructive] ni pida confirmación.
func assertReadOnly(t *testing.T, tool mcp.Tool) {
	t.Helper()
	a := tool.Annotations
	if a.ReadOnlyHint == nil || !*a.ReadOnlyHint {
		t.Errorf("%s: ReadOnlyHint debe ser true", tool.Name)
	}
	if a.DestructiveHint == nil || *a.DestructiveHint {
		t.Errorf("%s: DestructiveHint debe ser false", tool.Name)
	}
	if a.OpenWorldHint == nil || *a.OpenWorldHint {
		t.Errorf("%s: OpenWorldHint debe ser false", tool.Name)
	}
}

func TestTools_AreAnnotatedReadOnly(t *testing.T) {
	assertReadOnly(t, campaignsTool())
	assertReadOnly(t, insightsTool())
}
