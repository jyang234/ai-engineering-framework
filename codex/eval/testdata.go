package eval

// TestDocument represents a document in the test collection.
type TestDocument struct {
	ID      string   // e.g. "adr-001"
	Type    string   // pattern, failure, decision, context, code, doc
	Title   string
	Content string
	Tags    []string
	Scope   string   // global or project
}

// TestQuery represents a ground truth query with expected results.
type TestQuery struct {
	ID          string   // e.g. "q-01"
	Query       string   // natural language query
	Category    string   // semantic, keyword, hybrid-advantage
	RelevantIDs []string // ordered list of relevant document IDs
}

// TestCollection holds the full test corpus and ground truth.
type TestCollection struct {
	Documents []TestDocument
	Queries   []TestQuery
}

// SearchResultFromMCP represents a search result as returned through the MCP protocol.
type SearchResultFromMCP struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Tags       []string `json:"tags,omitempty"`
	Scope      string   `json:"scope"`
	Score      float64  `json:"score"`
	Highlights []string `json:"highlights,omitempty"`
}

// ItemFromMCP represents an item retrieved via recall_get through MCP.
type ItemFromMCP struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
	Scope   string   `json:"scope"`
}

// NewPayFlowCollection returns the standard PayFlow test collection.
func NewPayFlowCollection() TestCollection {
	return TestCollection{
		Documents: PayFlowDocuments(),
		Queries:   PayFlowQueries(),
	}
}
