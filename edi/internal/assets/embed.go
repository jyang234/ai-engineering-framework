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

//go:embed subagents/*.md
var Subagents embed.FS
