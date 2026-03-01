package guard

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// BuildTagPolicy injects missing build tags into go test/build/run commands.
type BuildTagPolicy struct {
	tags []string
}

// NewBuildTagPolicy returns a policy that ensures the given tags are present.
func NewBuildTagPolicy(tags []string) *BuildTagPolicy {
	return &BuildTagPolicy{tags: tags}
}

func (b *BuildTagPolicy) Name() string { return "build-tags" }

func (b *BuildTagPolicy) EvalPreToolUse(_ context.Context, _ *HookContext, command string) *PolicyResult {
	if len(b.tags) == 0 {
		return nil
	}
	modified, newCommand := injectBuildTags(command, b.tags)
	if !modified {
		return nil
	}
	return &PolicyResult{ModifiedCommand: newCommand}
}

// goCommandRe matches go test/build/run anywhere in a string.
var goCommandRe = regexp.MustCompile(`\bgo\s+(test|build|run)\b`)

// clauseSplitRe splits shell commands on &&, ||, and ;
var clauseSplitRe = regexp.MustCompile(`\s*(&&|\|\||;)\s*`)

// injectBuildTags checks if any clause needs build tags injected.
// Returns (modified, newFullCommand).
func injectBuildTags(command string, tags []string) (bool, string) {
	if len(tags) == 0 {
		return false, command
	}

	// Split into clauses and delimiters
	delimiters := clauseSplitRe.FindAllString(command, -1)
	clauses := clauseSplitRe.Split(command, -1)

	anyModified := false
	for i, clause := range clauses {
		trimmed := strings.TrimSpace(clause)
		// Skip make commands
		if strings.HasPrefix(trimmed, "make ") || trimmed == "make" {
			continue
		}
		if goCommandRe.MatchString(trimmed) && !hasAllTags(trimmed, tags) {
			clauses[i] = injectTagsIntoClause(clause, tags)
			anyModified = true
		}
	}

	if !anyModified {
		return false, command
	}

	// Reassemble
	var b strings.Builder
	for i, clause := range clauses {
		b.WriteString(clause)
		if i < len(delimiters) {
			b.WriteString(delimiters[i])
		}
	}
	return true, b.String()
}

// hasAllTags checks if a command clause already contains all required tags.
func hasAllTags(clause string, tags []string) bool {
	for _, tag := range tags {
		re := regexp.MustCompile(`-tags[= ]+\S*\b` + regexp.QuoteMeta(tag) + `\b`)
		if !re.MatchString(clause) {
			return false
		}
	}
	return true
}

// injectTagsIntoClause inserts -tags "tag1,tag2" after the go subcommand.
func injectTagsIntoClause(clause string, tags []string) string {
	tagValue := strings.Join(tags, ",")
	var inject string
	if len(tags) > 1 {
		inject = fmt.Sprintf(` -tags "%s"`, tagValue)
	} else {
		inject = fmt.Sprintf(` -tags %s`, tagValue)
	}

	loc := goCommandRe.FindStringIndex(clause)
	if loc == nil {
		return clause
	}
	return clause[:loc[1]] + inject + clause[loc[1]:]
}
