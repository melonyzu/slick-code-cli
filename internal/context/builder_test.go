package projectcontext

import (
	"strings"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/workspace"
)

type runeEstimator struct{}

func (runeEstimator) Estimate(text string) int { return len([]rune(text)) }

func TestBudgetDoesNotOverspend(t *testing.T) {
	budget := NewBudget(5, runeEstimator{})
	if !budget.Add("abc") || budget.Add("def") {
		t.Fatalf("budget used=%d remaining=%d", budget.Used(), budget.Remaining())
	}
	if budget.Used() != 3 || budget.Remaining() != 2 {
		t.Fatalf("budget used=%d remaining=%d", budget.Used(), budget.Remaining())
	}
}

func TestBuilderPrioritizesProjectMetadataWithinBudget(t *testing.T) {
	project := workspace.Project{Root: "/repo", IsGit: true}
	files := []workspace.File{
		{Path: "large.txt", Language: "Text", Size: 1_000, Content: strings.Repeat("x", 1_000)},
		{Path: "README.md", Language: "Markdown", Size: 8, Content: "overview"},
	}
	snapshot := NewBuilder(300, runeEstimator{}).Build(project, files)
	if snapshot.Estimated > snapshot.Budget {
		t.Fatalf("snapshot exceeded budget: %+v", snapshot)
	}
	if !strings.Contains(snapshot.Text, `path="README.md"`) || strings.Contains(snapshot.Text, `path="large.txt"`) {
		t.Fatalf("unexpected context:\n%s", snapshot.Text)
	}
	if !snapshot.Truncated || snapshot.IncludedFiles != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestBuilderIncludesRepositoryState(t *testing.T) {
	project := workspace.Project{Root: "/repo", IsGit: true}
	snapshot := NewBuilder(1_000, nil).BuildWithRepository(project, nil, &RepositoryState{
		Branch: "feature/context", Head: "abc123", Clean: false, Changed: 2,
	})
	for _, expected := range []string{"Git branch: feature/context", "Git HEAD: abc123", "Git clean: false", "Git changed files: 2"} {
		if !strings.Contains(snapshot.Text, expected) {
			t.Fatalf("context missing %q:\n%s", expected, snapshot.Text)
		}
	}
}
