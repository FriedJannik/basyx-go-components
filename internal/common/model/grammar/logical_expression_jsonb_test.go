/*******************************************************************************
* Copyright (C) 2025 the Eclipse BaSyx Authors and Fraunhofer IESE
*
* Permission is hereby granted, free of charge, to any person obtaining
* a copy of this software and associated documentation files (the
* "Software"), to deal in the Software without restriction, including
* without limitation the rights to use, copy, modify, merge, publish,
* distribute, sublicense, and/or sell copies of the Software, and to
* permit persons to whom the Software is furnished to do so, subject to
* the following conditions:
*
* The above copyright notice and this permission notice shall be
* included in all copies or substantial portions of the Software.
*
* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
* NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
* LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
* OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
* WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*
* SPDX-License-Identifier: MIT
******************************************************************************/

// Package grammar defines the data structures for representing logical expressions with JSONB support.
// Author: Jannik Fried ( Fraunhofer IESE )
package grammar

import (
	"testing"
)

func TestParseAASQLFieldToJSONBPath_RegularColumns(t *testing.T) {
	tests := []struct {
		name          string
		field         string
		expectedPath  string
		expectedIndex int
		expectedJSONB bool
	}{
		{
			name:          "idShort regular column",
			field:         "$sm#idShort",
			expectedPath:  "id_short",
			expectedIndex: -1,
			expectedJSONB: false,
		},
		{
			name:          "id regular column",
			field:         "$sm#id",
			expectedPath:  "submodel_id",
			expectedIndex: -1,
			expectedJSONB: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, index, isJSONB := ParseAASQLFieldToJSONBPath(tt.field)
			if path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, path)
			}
			if index != tt.expectedIndex {
				t.Errorf("Expected index %d, got %d", tt.expectedIndex, index)
			}
			if isJSONB != tt.expectedJSONB {
				t.Errorf("Expected isJSONB %v, got %v", tt.expectedJSONB, isJSONB)
			}
		})
	}
}

func TestParseAASQLFieldToJSONBPath_SemanticID(t *testing.T) {
	tests := []struct {
		name          string
		field         string
		expectedPath  string
		expectedIndex int
		expectedJSONB bool
	}{
		{
			name:          "semanticId shorthand (keys[0].value)",
			field:         "$sm#semanticId",
			expectedPath:  "semantic_id->'keys'->0->>'value'",
			expectedIndex: 0,
			expectedJSONB: true,
		},
		{
			name:          "semanticId type",
			field:         "$sm#semanticId.type",
			expectedPath:  "semantic_id->>'type'",
			expectedIndex: -1,
			expectedJSONB: true,
		},
		{
			name:          "semanticId keys array value wildcard",
			field:         "$sm#semanticId.keys[].value",
			expectedPath:  "semantic_id->'keys'",
			expectedIndex: -1,
			expectedJSONB: true,
		},
		{
			name:          "semanticId keys array type wildcard",
			field:         "$sm#semanticId.keys[].type",
			expectedPath:  "semantic_id->'keys'",
			expectedIndex: -1,
			expectedJSONB: true,
		},
		{
			name:          "semanticId keys[0].value specific index",
			field:         "$sm#semanticId.keys[0].value",
			expectedPath:  "semantic_id->'keys'->0->>'value'",
			expectedIndex: 0,
			expectedJSONB: true,
		},
		{
			name:          "semanticId keys[1].type specific index",
			field:         "$sm#semanticId.keys[1].type",
			expectedPath:  "semantic_id->'keys'->1->>'type'",
			expectedIndex: 1,
			expectedJSONB: true,
		},
		{
			name:          "semanticId keys[5].value specific index",
			field:         "$sm#semanticId.keys[5].value",
			expectedPath:  "semantic_id->'keys'->5->>'value'",
			expectedIndex: 5,
			expectedJSONB: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, index, isJSONB := ParseAASQLFieldToJSONBPath(tt.field)
			if path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, path)
			}
			if index != tt.expectedIndex {
				t.Errorf("Expected index %d, got %d", tt.expectedIndex, index)
			}
			if isJSONB != tt.expectedJSONB {
				t.Errorf("Expected isJSONB %v, got %v", tt.expectedJSONB, isJSONB)
			}
		})
	}
}

func TestHandleComparison_RegularColumns(t *testing.T) {
	field := ModelStringPattern("$sm#idShort")
	value := StandardString("TestSubmodel")

	leftOperand := &Value{Field: &field}
	rightOperand := &Value{StrVal: &value}

	expr, err := HandleComparison(leftOperand, rightOperand, "$eq")
	if err != nil {
		t.Fatalf("HandleComparison failed: %v", err)
	}

	if expr == nil {
		t.Fatal("Expected non-nil expression")
	}

	// Successfully created expression for regular column
}

func TestHandleComparison_SemanticIDShorthand(t *testing.T) {
	field := ModelStringPattern("$sm#semanticId")
	value := StandardString("https://example.com/semantic")

	leftOperand := &Value{Field: &field}
	rightOperand := &Value{StrVal: &value}

	expr, err := HandleComparison(leftOperand, rightOperand, "$eq")
	if err != nil {
		t.Fatalf("HandleComparison failed: %v", err)
	}

	if expr == nil {
		t.Fatal("Expected non-nil expression")
	}

	// Successfully created expression for semantic ID shorthand
	// Should map to semantic_id->'keys'->0->>'value'
}

func TestHandleComparison_SemanticIDSpecificIndex(t *testing.T) {
	field := ModelStringPattern("$sm#semanticId.keys[2].value")
	value := StandardString("Submodel")

	leftOperand := &Value{Field: &field}
	rightOperand := &Value{StrVal: &value}

	expr, err := HandleComparison(leftOperand, rightOperand, "$eq")
	if err != nil {
		t.Fatalf("HandleComparison failed: %v", err)
	}

	if expr == nil {
		t.Fatal("Expected non-nil expression")
	}

	// Successfully created expression with specific array index
	// Should map to semantic_id->'keys'->2->>'value'
}

func TestHandleComparison_ArrayWildcard(t *testing.T) {
	field := ModelStringPattern("$sm#semanticId.keys[].value")
	value := StandardString("https://example.com/semantic")

	leftOperand := &Value{Field: &field}
	rightOperand := &Value{StrVal: &value}

	expr, err := HandleComparison(leftOperand, rightOperand, "$eq")
	if err != nil {
		t.Fatalf("HandleComparison failed: %v", err)
	}

	if expr == nil {
		t.Fatal("Expected non-nil expression")
	}

	// Successfully created expression for array wildcard
	// Should use semantic_id @? '$.keys[*] ? (@.value == "...")'
}

func TestLogicalExpression_SimpleEquality(t *testing.T) {
	field := ModelStringPattern("$sm#idShort")
	value := StandardString("TestSubmodel")

	expr := LogicalExpression{
		Eq: []Value{
			{Field: &field},
			{StrVal: &value},
		},
	}

	result, err := expr.EvaluateToExpression()
	if err != nil {
		t.Fatalf("EvaluateToExpression failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestLogicalExpression_AndCondition(t *testing.T) {
	field1 := ModelStringPattern("$sm#idShort")
	value1 := StandardString("TestSubmodel")
	field2 := ModelStringPattern("$sm#semanticId")
	value2 := StandardString("https://example.com/semantic")

	expr := LogicalExpression{
		And: []LogicalExpression{
			{
				Eq: []Value{
					{Field: &field1},
					{StrVal: &value1},
				},
			},
			{
				Eq: []Value{
					{Field: &field2},
					{StrVal: &value2},
				},
			},
		},
	}

	result, err := expr.EvaluateToExpression()
	if err != nil {
		t.Fatalf("EvaluateToExpression failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Successfully created AND expression
}

func TestLogicalExpression_NumericComparison(t *testing.T) {
	field := ModelStringPattern("$sm#id")
	num := 100.0

	expr := LogicalExpression{
		Gt: []Value{
			{Field: &field},
			{NumVal: &num},
		},
	}

	result, err := expr.EvaluateToExpression()
	if err != nil {
		t.Fatalf("EvaluateToExpression failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Successfully created numeric comparison
}
