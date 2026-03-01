package guard

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Build tag injection tests
// ---------------------------------------------------------------------------

func TestInjectBuildTags_Missing(t *testing.T) {
	modified, result := injectBuildTags("go test ./...", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != "go test -tags fts5 ./..." {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_Present(t *testing.T) {
	modified, _ := injectBuildTags("go test -tags fts5 ./...", []string{"fts5"})
	if modified {
		t.Fatal("expected no modification")
	}
}

func TestInjectBuildTags_PresentEquals(t *testing.T) {
	modified, _ := injectBuildTags("go test -tags=fts5 ./...", []string{"fts5"})
	if modified {
		t.Fatal("expected no modification")
	}
}

func TestInjectBuildTags_PresentComma(t *testing.T) {
	modified, _ := injectBuildTags(`go test -tags "fts5,integration" ./...`, []string{"fts5"})
	if modified {
		t.Fatal("expected no modification")
	}
}

func TestInjectBuildTags_CompoundCommand(t *testing.T) {
	modified, result := injectBuildTags("cd codex && go test ./...", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != "cd codex && go test -tags fts5 ./..." {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_MakeSkip(t *testing.T) {
	modified, _ := injectBuildTags("make test", []string{"fts5"})
	if modified {
		t.Fatal("expected no modification for make")
	}
}

func TestInjectBuildTags_CompoundWithMake(t *testing.T) {
	modified, result := injectBuildTags("make foo && go test ./...", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification for go test clause")
	}
	if !strings.Contains(result, "make foo") {
		t.Fatal("make clause should be preserved")
	}
	if !strings.Contains(result, "go test -tags fts5 ./...") {
		t.Fatalf("go test clause not modified: %q", result)
	}
}

func TestInjectBuildTags_MultipleTags(t *testing.T) {
	modified, result := injectBuildTags("go test ./...", []string{"fts5", "integration"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != `go test -tags "fts5,integration" ./...` {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_GoBuild(t *testing.T) {
	modified, result := injectBuildTags("go build -o bin/edi ./cmd/edi", []string{"fts5"})
	if !modified {
		t.Fatal("expected modification")
	}
	if result != "go build -tags fts5 -o bin/edi ./cmd/edi" {
		t.Fatalf("got %q", result)
	}
}

func TestInjectBuildTags_NoTags(t *testing.T) {
	modified, _ := injectBuildTags("go test ./...", nil)
	if modified {
		t.Fatal("expected no modification when no tags configured")
	}
}

// ---------------------------------------------------------------------------
// hasAllTags tests
// ---------------------------------------------------------------------------

func TestHasAllTags_SinglePresent(t *testing.T) {
	if !hasAllTags("go test -tags fts5 ./...", []string{"fts5"}) {
		t.Fatal("should find fts5")
	}
}

func TestHasAllTags_SingleMissing(t *testing.T) {
	if hasAllTags("go test ./...", []string{"fts5"}) {
		t.Fatal("should not find fts5")
	}
}

func TestHasAllTags_MultipleAllPresent(t *testing.T) {
	if !hasAllTags(`go test -tags "fts5,integration" ./...`, []string{"fts5", "integration"}) {
		t.Fatal("should find both")
	}
}

func TestHasAllTags_MultipleOneMissing(t *testing.T) {
	if hasAllTags("go test -tags fts5 ./...", []string{"fts5", "integration"}) {
		t.Fatal("should not find integration")
	}
}
