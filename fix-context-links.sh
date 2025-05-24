#!/bin/bash

# Fix context handler Links field issues
FILE="/Users/seancorkum/projects/devops-mcp/apps/rest-api/internal/api/context/handlers.go"

# Create a temporary fixed version
cp "$FILE" "$FILE.tmp"

# Replace all occurrences of result.Links pattern
sed -i '' '
/if result\.Links == nil {/,/c\.JSON(http\.StatusOK, result)/ {
    s/if result\.Links == nil {/response := \&contextResponse{/
    s/result\.Links = make(map\[string\]string)/\tContext: result,\n\t\tLinks: map[string]string{/
    s/}/\t},/
    s/result\.Links\["self"\] = .*$/\t\t\t"self":    "\/api\/v1\/contexts\/" + result.ID,/
    s/result\.Links\["summary"\] = .*$/\t\t\t"summary": "\/api\/v1\/contexts\/" + result.ID + "\/summary",/
    s/result\.Links\["search"\] = .*$/\t\t\t"search":  "\/api\/v1\/contexts\/" + result.ID + "\/search",/
    s/c\.JSON(http\.StatusOK, result)/\t}\n\t\n\tc.JSON(http.StatusOK, gin.H{\n\t\t"data":       response,\n\t\t"request_id": c.GetString("RequestID"),\n\t\t"timestamp":  time.Now().UTC(),\n\t})/
}
' "$FILE.tmp"

# Check if changes were made
if diff -q "$FILE" "$FILE.tmp" >/dev/null; then
    echo "No changes needed"
    rm "$FILE.tmp"
else
    mv "$FILE.tmp" "$FILE"
    echo "Context handler fixed"
fi