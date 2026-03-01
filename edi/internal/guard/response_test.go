package guard

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPreToolUseResponse_ModifiedOnly(t *testing.T) {
	resp := BuildPreToolUseResponse("go test -tags fts5 ./...", nil, true)
	data, _ := json.Marshal(resp)
	s := string(data)
	if !strings.Contains(s, "updatedInput") {
		t.Error("missing updatedInput")
	}
	if strings.Contains(s, "additionalContext") {
		t.Error("should not have additionalContext")
	}
}

func TestBuildPreToolUseResponse_AdvisoryOnly(t *testing.T) {
	resp := BuildPreToolUseResponse("", []string{"you're stuck"}, false)
	data, _ := json.Marshal(resp)
	s := string(data)
	if strings.Contains(s, "updatedInput") {
		t.Error("should not have updatedInput")
	}
	if !strings.Contains(s, "additionalContext") {
		t.Error("missing additionalContext")
	}
}

func TestBuildPreToolUseResponse_Both(t *testing.T) {
	resp := BuildPreToolUseResponse("go test -tags fts5 ./...", []string{"you're stuck"}, true)
	data, _ := json.Marshal(resp)
	s := string(data)
	if !strings.Contains(s, "updatedInput") {
		t.Error("missing updatedInput")
	}
	if !strings.Contains(s, "additionalContext") {
		t.Error("missing additionalContext")
	}
}
