# Task: JSON Validation Middleware

Implement HTTP middleware that validates incoming JSON request bodies against a schema.

## Requirements

Implement the following in `validator.go`:

1. **`ValidationRule`** struct with fields:
   - `Field string` — JSON field name to validate
   - `Required bool` — whether the field must be present
   - `Type string` — expected type: "string", "number", "bool", "array", "object"
   - `MinLength int` — minimum string length (for type "string", 0 means no minimum)
   - `MaxLength int` — maximum string length (for type "string", 0 means no maximum)

2. **`NewValidator(rules []ValidationRule, maxBodySize int64) *Validator`** that creates a validator with the given rules and maximum request body size in bytes.

3. **`(*Validator) Middleware(next http.Handler) http.Handler`** that returns middleware which:
   - Rejects requests without `Content-Type: application/json` header (415 status)
   - Limits request body to `maxBodySize` bytes (413 status if exceeded)
   - Parses the JSON body
   - Validates all fields against the rules (400 status with error details on failure)
   - **Ensures the request body is always closed**, even on error paths
   - Stores the parsed body in the request context for downstream handlers
   - Calls `next.ServeHTTP` on success

4. **`ParsedBody(ctx context.Context) map[string]interface{}`** to retrieve the validated body from context.

## Constraints

- Use only the Go standard library
- The request body must ALWAYS be closed (use defer)
- Must enforce max body size to prevent denial of service
- Return structured JSON error responses: `{"error": "description"}`
- HTTP status codes must be correct (400, 413, 415)

## Example Usage

```go
rules := []ValidationRule{
    {Field: "name", Required: true, Type: "string", MinLength: 1},
    {Field: "email", Required: true, Type: "string"},
    {Field: "age", Required: false, Type: "number"},
}
v := NewValidator(rules, 1<<20) // 1MB max
mux.Handle("/users", v.Middleware(userHandler))
```
