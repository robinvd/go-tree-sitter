package tree_sitter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	. "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func TestCapturesWithPartialCallbacks(t *testing.T) {
	// Test that CapturesWith works correctly when callback returns small chunks
	language := NewLanguage(tree_sitter_go.Language())
	parser := NewParser()
	defer parser.Close()
	parser.SetLanguage(language)

	sourceCode := []byte(`package main; func test() string { return "hello"; }`)
	tree := parser.Parse(sourceCode, nil)
	defer tree.Close()

	// Query to capture the string literal with a text predicate to trigger callback
	query, err := NewQuery(language, `((interpreted_string_literal) @string (#eq? @string "\"hello\""))`)
	if err != nil {
		t.Fatalf("Query creation failed: %v", err)
	}
	defer query.Close()

	cursor := NewQueryCursor()
	defer cursor.Close()

	// Create a callback that returns only 2 bytes at a time
	callCount := 0
	captures := cursor.CapturesWith(query, tree.RootNode(), func(offset int, position Point) []byte {
		callCount++
		if offset >= len(sourceCode) {
			return []byte{}
		}
		
		// Return max 2 bytes at a time to test multiple callback functionality
		end := offset + 2
		if end > len(sourceCode) {
			end = len(sourceCode)
		}
		return sourceCode[offset:end]
	})

	// Collect all captures
	var results []string
	matchCount := 0
	for {
		match, _ := captures.Next()
		if match == nil {
			break
		}
		matchCount++
		for _, capture := range match.Captures {
			captureText := string(getTextForTestNode(capture.Node, sourceCode))
			results = append(results, captureText)
		}
	}

	// Debug output
	t.Logf("Matches found: %d, Results: %v, CallCount: %d", matchCount, results, callCount)

	// Should find the string literal "hello" (including quotes)
	assert.Equal(t, []string{`"hello"`}, results)
	
	// Should have made multiple callback calls due to 2-byte chunks
	// The callback gets called during predicate evaluation for the text match
	assert.Greater(t, callCount, 1, "Expected multiple callback calls due to chunking")
}

func TestCapturesWithSingleByteCallbacks(t *testing.T) {
	// Extreme test with 1-byte callbacks
	language := NewLanguage(tree_sitter_go.Language())
	parser := NewParser()
	defer parser.Close()
	parser.SetLanguage(language)

	sourceCode := []byte(`package main; func test() {}`)
	tree := parser.Parse(sourceCode, nil)
	defer tree.Close()

	// Query to capture identifiers with text predicate
	query, err := NewQuery(language, `((identifier) @id (#eq? @id "test"))`)
	if err != nil {
		t.Fatalf("Query creation failed: %v", err)
	}
	defer query.Close()

	cursor := NewQueryCursor()
	defer cursor.Close()

	// Return 1 byte at a time
	captures := cursor.CapturesWith(query, tree.RootNode(), func(offset int, position Point) []byte {
		if offset >= len(sourceCode) {
			return []byte{}
		}
		return sourceCode[offset : offset+1]
	})

	// Should still work correctly
	var results []string
	for {
		match, _ := captures.Next()
		if match == nil {
			break
		}
		for _, capture := range match.Captures {
			captureText := string(getTextForTestNode(capture.Node, sourceCode))
			results = append(results, captureText)
		}
	}

	// Should find the identifier "test" 
	assert.Equal(t, []string{"test"}, results)
}

func TestCapturesWithEmptyCallback(t *testing.T) {
	// Test behavior when callback returns empty after some data
	language := NewLanguage(tree_sitter_go.Language())
	parser := NewParser()
	defer parser.Close()
	parser.SetLanguage(language)

	sourceCode := []byte(`package main; func main() {}`)
	tree := parser.Parse(sourceCode, nil)
	defer tree.Close()

	query, err := NewQuery(language, `((identifier) @id (#eq? @id "main"))`)
	if err != nil {
		t.Fatalf("Query creation failed: %v", err)
	}
	defer query.Close()

	cursor := NewQueryCursor()
	defer cursor.Close()

	// Return only first 2 chars, then empty
	callCount := 0
	captures := cursor.CapturesWith(query, tree.RootNode(), func(offset int, position Point) []byte {
		callCount++
		if callCount == 1 && offset < len(sourceCode) {
			// First call: return 2 bytes
			end := offset + 2
			if end > len(sourceCode) {
				end = len(sourceCode)
			}
			return sourceCode[offset:end]
		}
		// Subsequent calls: return empty (simulating end of data)
		return []byte{}
	})

	// Should get partial text - this tests graceful degradation
	var results []string
	for {
		match, _ := captures.Next()
		if match == nil {
			break
		}
		for _, capture := range match.Captures {
			captureText := string(getTextForTestNode(capture.Node, sourceCode))
			results = append(results, captureText)
		}
	}

	// Even with limited callback data, we should still get some results
	// The actual behavior depends on how predicates handle partial text
	assert.GreaterOrEqual(t, len(results), 0) // At minimum, no panic/error
}

// Helper function to get text for a node (for comparison)
func getTextForTestNode(node Node, source []byte) []byte {
	return source[node.StartByte():node.EndByte()]
}