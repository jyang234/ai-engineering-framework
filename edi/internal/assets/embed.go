package assets

import "embed"

//go:embed agents/*.md
var Agents embed.FS

//go:embed commands/*.md
var Commands embed.FS

//go:embed skills/edi-core/SKILL.md
var EdiCoreSkill embed.FS

//go:embed skills/retrieval-judge/SKILL.md
var RetrievalJudgeSkill embed.FS

//go:embed skills/coding/SKILL.md
var CodingSkill embed.FS

//go:embed skills/testing/SKILL.md
var TestingSkill embed.FS

//go:embed skills/scaffolding-tests/SKILL.md
var ScaffoldingTestsSkill embed.FS

//go:embed skills/refactoring-planning/SKILL.md
var RefactoringPlanningSkill embed.FS

//go:embed subagents/*.md
var Subagents embed.FS

//go:embed ralph/*
var Ralph embed.FS
