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

// Package grammar defines the data structures for representing logical expressions in the grammar model.
// Author: Aaron Zielstorff ( Fraunhofer IESE ), Jannik Fried ( Fraunhofer IESE )
package grammar

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
)

// LogicalExpression represents a logical expression tree for AAS access control rules.
//
// This structure supports complex logical conditions that can be evaluated against AAS elements.
// It combines comparison operations (eq, ne, gt, ge, lt, le), string operations (contains,
// starts-with, ends-with, regex), and logical operators (AND, OR, NOT) to form sophisticated
// access control rules. Expressions can be nested to create complex conditional logic.
//
// Only one operation field should be set per LogicalExpression instance. The structure can
// be converted to SQL WHERE clauses using the EvaluateToExpression method.
//
// Logical operators:
//   - $and: All conditions must be true (requires at least 2 expressions)
//   - $or: At least one condition must be true (requires at least 2 expressions)
//   - $not: Negates the nested expression
//
// Comparison operators: $eq, $ne, $gt, $ge, $lt, $le
// String operators: $contains, $starts-with, $ends-with, $regex
// Boolean: Direct boolean value evaluation
//
// Example JSON:
//
//	{"$and": [
//	  {"$eq": ["$sm#idShort", "MySubmodel"]},
//	  {"$gt": ["$sme.temperature#value", "100"]}
//	]}
type LogicalExpression struct {
	// And corresponds to the JSON schema field "$and".
	And []LogicalExpression `json:"$and,omitempty" yaml:"$and,omitempty" mapstructure:"$and,omitempty"`

	// Boolean corresponds to the JSON schema field "$boolean".
	Boolean *bool `json:"$boolean,omitempty" yaml:"$boolean,omitempty" mapstructure:"$boolean,omitempty"`

	// Contains corresponds to the JSON schema field "$contains".
	Contains StringItems `json:"$contains,omitempty" yaml:"$contains,omitempty" mapstructure:"$contains,omitempty"`

	// EndsWith corresponds to the JSON schema field "$ends-with".
	EndsWith StringItems `json:"$ends-with,omitempty" yaml:"$ends-with,omitempty" mapstructure:"$ends-with,omitempty"`

	// Eq corresponds to the JSON schema field "$eq".
	Eq ComparisonItems `json:"$eq,omitempty" yaml:"$eq,omitempty" mapstructure:"$eq,omitempty"`

	// Ge corresponds to the JSON schema field "$ge".
	Ge ComparisonItems `json:"$ge,omitempty" yaml:"$ge,omitempty" mapstructure:"$ge,omitempty"`

	// Gt corresponds to the JSON schema field "$gt".
	Gt ComparisonItems `json:"$gt,omitempty" yaml:"$gt,omitempty" mapstructure:"$gt,omitempty"`

	// Le corresponds to the JSON schema field "$le".
	Le ComparisonItems `json:"$le,omitempty" yaml:"$le,omitempty" mapstructure:"$le,omitempty"`

	// Lt corresponds to the JSON schema field "$lt".
	Lt ComparisonItems `json:"$lt,omitempty" yaml:"$lt,omitempty" mapstructure:"$lt,omitempty"`

	// Match corresponds to the JSON schema field "$match".
	Match []MatchExpression `json:"$match,omitempty" yaml:"$match,omitempty" mapstructure:"$match,omitempty"`

	// Ne corresponds to the JSON schema field "$ne".
	Ne ComparisonItems `json:"$ne,omitempty" yaml:"$ne,omitempty" mapstructure:"$ne,omitempty"`

	// Not corresponds to the JSON schema field "$not".
	Not *LogicalExpression `json:"$not,omitempty" yaml:"$not,omitempty" mapstructure:"$not,omitempty"`

	// Or corresponds to the JSON schema field "$or".
	Or []LogicalExpression `json:"$or,omitempty" yaml:"$or,omitempty" mapstructure:"$or,omitempty"`

	// Regex corresponds to the JSON schema field "$regex".
	Regex StringItems `json:"$regex,omitempty" yaml:"$regex,omitempty" mapstructure:"$regex,omitempty"`

	// StartsWith corresponds to the JSON schema field "$starts-with".
	StartsWith StringItems `json:"$starts-with,omitempty" yaml:"$starts-with,omitempty" mapstructure:"$starts-with,omitempty"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for LogicalExpression.
//
// This custom unmarshaler validates that logical operator arrays contain the required
// minimum number of elements:
//   - $and requires at least 2 expressions
//   - $or requires at least 2 expressions
//   - $match requires at least 1 expression
//
// These constraints ensure that logical operations are meaningful and properly formed.
//
// Parameters:
//   - value: JSON byte slice containing the logical expression to unmarshal
//
// Returns:
//   - error: An error if the JSON is invalid or if array constraints are violated.
//     Returns nil on successful unmarshaling and validation.
func (le *LogicalExpression) UnmarshalJSON(value []byte) error {
	type Plain LogicalExpression
	var plain Plain
	if err := json.Unmarshal(value, &plain); err != nil {
		return err
	}
	if plain.And != nil && len(plain.And) < 2 {
		return fmt.Errorf("field %s length: must be >= %d", "$and", 2)
	}
	if plain.Match != nil && len(plain.Match) < 1 {
		return fmt.Errorf("field %s length: must be >= %d", "$match", 1)
	}
	if plain.Or != nil && len(plain.Or) < 2 {
		return fmt.Errorf("field %s length: must be >= %d", "$or", 2)
	}
	*le = LogicalExpression(plain)
	return nil
}

// EvaluateToExpression converts the logical expression tree into a goqu SQL expression.
//
// This method traverses the logical expression tree and constructs a corresponding SQL
// WHERE clause expression that can be used with the goqu query builder. It handles all
// supported comparison operations, logical operators (AND, OR, NOT), and nested expressions.
//
// The method supports special handling for AAS-specific fields, particularly semantic IDs,
// where additional constraints (like position = 0) may be added to the generated SQL.
//
// Supported operations:
//   - Comparison: $eq, $ne, $gt, $ge, $lt, $le
//   - Logical: $and (all true), $or (any true), $not (negation)
//   - Boolean: Direct boolean literal evaluation
//
// Returns:
//   - exp.Expression: A goqu expression that can be used in SQL WHERE clauses
//   - error: An error if the expression is invalid, has no valid operation, or if
//     evaluation of nested expressions fails
func (le *LogicalExpression) EvaluateToExpression() (exp.Expression, error) {
	// Handle comparison operations
	if len(le.Eq) > 0 {
		return le.evaluateComparison(le.Eq, "$eq")
	}
	if len(le.Ne) > 0 {
		return le.evaluateComparison(le.Ne, "$ne")
	}
	if len(le.Gt) > 0 {
		return le.evaluateComparison(le.Gt, "$gt")
	}
	if len(le.Ge) > 0 {
		return le.evaluateComparison(le.Ge, "$ge")
	}
	if len(le.Lt) > 0 {
		return le.evaluateComparison(le.Lt, "$lt")
	}
	if len(le.Le) > 0 {
		return le.evaluateComparison(le.Le, "$le")
	}

	// Handle logical operations
	if len(le.And) > 0 {
		var expressions []exp.Expression
		for i, nestedExpr := range le.And {
			expr, err := nestedExpr.EvaluateToExpression()
			if err != nil {
				return nil, fmt.Errorf("error evaluating AND condition at index %d: %w", i, err)
			}
			expressions = append(expressions, expr)
		}
		return goqu.And(expressions...), nil
	}

	if len(le.Or) > 0 {
		var expressions []exp.Expression
		for i, nestedExpr := range le.Or {
			expr, err := nestedExpr.EvaluateToExpression()
			if err != nil {
				return nil, fmt.Errorf("error evaluating OR condition at index %d: %w", i, err)
			}
			expressions = append(expressions, expr)
		}
		return goqu.Or(expressions...), nil
	}

	if le.Not != nil {
		expr, err := le.Not.EvaluateToExpression()
		if err != nil {
			return nil, fmt.Errorf("error evaluating NOT condition: %w", err)
		}
		return goqu.L("NOT (?)", expr), nil
	}

	// Handle boolean literal
	if le.Boolean != nil {
		return goqu.L("?", *le.Boolean), nil
	}

	return nil, fmt.Errorf("logical expression has no valid operation")
}

// evaluateComparison evaluates a comparison operation with the given operands
func (le *LogicalExpression) evaluateComparison(operands []Value, operation string) (exp.Expression, error) {
	if len(operands) != 2 {
		return nil, fmt.Errorf("comparison operation %s requires exactly 2 operands, got %d", operation, len(operands))
	}

	leftOperand := &operands[0]
	rightOperand := &operands[1]

	return HandleComparison(leftOperand, rightOperand, operation)
}

// ParseAASQLFieldToJSONBPath translates AAS query language field names to JSONB path expressions.
//
// This function maps AAS-specific field references (like $sm#idShort, $sm#semanticId) to their
// corresponding JSONB paths for querying data stored in PostgreSQL JSONB columns with GORM.
//
// Supported field mappings for GORM/JSONB structure:
//
// Submodel fields ($sm#):
//   - $sm#idShort -> id_short (regular column)
//   - $sm#id -> submodel_id (regular column)
//   - $sm#semanticId -> semantic_id->'keys'->0->>'value' (shorthand for keys[0].value)
//   - $sm#semanticId.type -> semantic_id->>'type'
//   - $sm#semanticId.keys[].value -> semantic_id->'keys' (for array operations)
//   - $sm#semanticId.keys[].type -> semantic_id->'keys' (for array operations)
//   - $sm#semanticId.keys[N].value -> semantic_id->'keys'->N->>'value'
//   - $sm#semanticId.keys[N].type -> semantic_id->'keys'->N->>'type'
//
// Submodel element fields ($sme#):
//   - $sme#semanticId -> submodel_elements @? path for semanticId.keys[0].value (uses @? operator)
//   - $sme#semanticId.keys[].value -> submodel_elements @? path (for array wildcard)
//   - $sme#semanticId.keys[N].value -> submodel_elements @? path (for specific index)
//
// Parameters:
//   - field: AAS query language field reference string
//
// Returns:
//   - jsonbPath: The JSONB path expression for the field
//   - arrayIndex: The array index if querying a specific position, or -1 for wildcard
//   - isJSONB: true if this is a JSONB column access, false for regular column
func ParseAASQLFieldToJSONBPath(field string) (jsonbPath string, arrayIndex int, isJSONB bool) {
	// Handle $sm# (submodel) fields
	switch field {
	case "$sm#idShort":
		return "id_short", -1, false
	case "$sm#id":
		return "submodel_id", -1, false
	case "$sm#semanticId": // Shorthand for keys[0].value
		return "semantic_id->'keys'->0->>'value'", 0, true
	case "$sm#semanticId.type":
		return "semantic_id->>'type'", -1, true
	case "$sm#semanticId.keys[].value":
		return "semantic_id->'keys'", -1, true
	case "$sm#semanticId.keys[].type":
		return "semantic_id->'keys'", -1, true
	}

	// Handle specific array index access: $sm#semanticId.keys[N].value or .type
	if strings.HasPrefix(field, "$sm#semanticId.keys[") {
		start := strings.Index(field, "[")
		end := strings.Index(field, "]")

		if start != -1 && end != -1 && end > start+1 {
			indexStr := field[start+1 : end]
			index, err := strconv.Atoi(indexStr)
			if err == nil {
				// Valid numeric index
				if strings.HasSuffix(field, "].value") {
					return fmt.Sprintf("semantic_id->'keys'->%d->>'value'", index), index, true
				} else if strings.HasSuffix(field, "].type") {
					return fmt.Sprintf("semantic_id->'keys'->%d->>'type'", index), index, true
				}
			}
		}
	}

	// Handle $sme# (submodel element) fields - these query into the submodel_elements JSONB array
	if strings.HasPrefix(field, "$sme#") {
		// For $sme# fields, return a marker that will be handled specially
		// The actual JSONB path query will be built in buildJSONBArrayComparisonExpression
		return field, -1, true
	}

	return field, -1, false
}

// HandleComparison builds a SQL comparison expression from two Value operands for GORM/JSONB.
//
// This function handles all combinations of operand types: field-to-field, field-to-value,
// value-to-field, and value-to-value comparisons. It uses PostgreSQL JSONB operators to
// query data stored in JSONB columns, and validates that value-to-value comparisons have
// matching types.
//
// Special handling for semantic IDs with JSONB:
//   - Shorthand references ($sm#semanticId) map to semantic_id->'keys'->0->>'value'
//   - Specific key references ($sm#semanticId.keys[N].value) map to semantic_id->'keys'->N->>'value'
//   - Wildcard references ($sm#semanticId.keys[].value) use JSONB path queries for array matching
//
// Parameters:
//   - leftOperand: The left side of the comparison (field or value)
//   - rightOperand: The right side of the comparison (field or value)
//   - operation: The comparison operator ($eq, $ne, $gt, $ge, $lt, $le)
//
// Returns:
//   - exp.Expression: A goqu expression representing the comparison using JSONB operators
//   - error: An error if the operands are invalid, types don't match, or the operation is unsupported
func HandleComparison(leftOperand, rightOperand *Value, operation string) (exp.Expression, error) {
	// Validate value-to-value comparisons have matching types
	if !leftOperand.IsField() && !rightOperand.IsField() {
		if leftOperand.GetValueType() != rightOperand.GetValueType() {
			return nil, fmt.Errorf("cannot compare different value types: %s and %s",
				leftOperand.GetValueType(), rightOperand.GetValueType())
		}
	}

	// Check if we have array wildcard queries (keys[]) or submodel element queries ($sme#)
	leftIsArrayWildcard, rightIsArrayWildcard := false, false
	leftIsSME, rightIsSME := false, false

	if leftOperand.IsField() && leftOperand.Field != nil {
		field := string(*leftOperand.Field)
		leftIsArrayWildcard = (field == "$sm#semanticId.keys[].value" || field == "$sm#semanticId.keys[].type")
		leftIsSME = strings.HasPrefix(field, "$sme#")
	}
	if rightOperand.IsField() && rightOperand.Field != nil {
		field := string(*rightOperand.Field)
		rightIsArrayWildcard = (field == "$sm#semanticId.keys[].value" || field == "$sm#semanticId.keys[].type")
		rightIsSME = strings.HasPrefix(field, "$sme#")
	}

	// Handle array wildcard queries and submodel element queries using JSONB path expressions
	if (leftIsArrayWildcard || leftIsSME) && !rightOperand.IsField() {
		return buildJSONBArrayComparisonExpression(leftOperand, rightOperand, operation, true)
	}
	if (rightIsArrayWildcard || rightIsSME) && !leftOperand.IsField() {
		return buildJSONBArrayComparisonExpression(rightOperand, leftOperand, operation, false)
	}

	// Convert both operands to SQL components
	leftSQL, err := toJSONBSQLComponent(leftOperand, "left")
	if err != nil {
		return nil, err
	}

	rightSQL, err := toJSONBSQLComponent(rightOperand, "right")
	if err != nil {
		return nil, err
	}

	// Build the comparison expression
	return buildComparisonExpression(leftSQL, rightSQL, operation)
}

// ToGORMWhere converts a LogicalExpression to a GORM-compatible WHERE clause.
//
// This helper function evaluates the logical expression and converts it to a format
// that GORM can use directly. It returns a SQL string and its parameters that can
// be passed to GORM's Where() method.
//
// Returns:
//   - sql: The SQL WHERE clause string
//   - args: The parameters for the SQL string
//   - error: An error if the expression cannot be evaluated or converted
func (le *LogicalExpression) ToGORMWhere() (sql string, args []interface{}, err error) {
	expr, err := le.EvaluateToExpression()
	if err != nil {
		return "", nil, err
	}

	// Use goqu to convert expression to SQL
	dialect := goqu.Dialect("postgres")
	sqlBuilder := dialect.From("submodels").Where(expr).Select(goqu.L("1"))

	query, params, err := sqlBuilder.ToSQL()
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert expression to SQL: %w", err)
	}

	// Extract just the WHERE clause part
	// The query will be something like: SELECT 1 FROM "submodels" WHERE ...
	whereIndex := strings.Index(query, "WHERE")
	if whereIndex == -1 {
		return "", nil, fmt.Errorf("failed to extract WHERE clause from query")
	}

	whereClause := query[whereIndex+6:] // Skip "WHERE "

	return whereClause, params, nil
}

// buildJSONBArrayComparisonExpression builds a JSONB array comparison using PostgreSQL path queries.
//
// This handles:
// 1. Wildcard array queries like $sm#semanticId.keys[].value
// 2. Submodel element queries like $sme#semanticId.keys[N].value
//
// Both use PostgreSQL's JSONB path query operators (@?) to match elements in arrays.
//
// Parameters:
//   - fieldOperand: The field operand containing the array reference
//   - valueOperand: The value to compare against
//   - operation: The comparison operator
//   - _ : unused parameter for API consistency
//
// Returns:
//   - exp.Expression: A JSONB path query expression
//   - error: An error if the expression cannot be built
func buildJSONBArrayComparisonExpression(fieldOperand, valueOperand *Value, operation string, _ bool) (exp.Expression, error) {
	if fieldOperand.Field == nil {
		return nil, fmt.Errorf("field operand is nil")
	}

	field := string(*fieldOperand.Field)
	value := valueOperand.GetValue()

	// Handle $sme# (submodel element) queries
	if strings.HasPrefix(field, "$sme#") {
		return buildSubmodelElementJSONBQuery(field, value, operation)
	}

	// Handle $sm# semantic ID array wildcards
	var property string
	switch field {
	case "$sm#semanticId.keys[].value":
		property = "value"
	case "$sm#semanticId.keys[].type":
		property = "type"
	default:
		return nil, fmt.Errorf("unsupported array wildcard field: %s", field)
	}

	// Build the JSONB path query based on operation
	var pathQuery string
	switch operation {
	case "$eq":
		pathQuery = fmt.Sprintf("$.keys[*] ? (@.%s == \"%v\")", property, value)
	case "$ne":
		// For not equals, we need to check that no element matches
		pathQuery = fmt.Sprintf("$.keys[*] ? (@.%s != \"%v\")", property, value)
	case "$gt":
		pathQuery = fmt.Sprintf("$.keys[*] ? (@.%s > %v)", property, value)
	case "$ge":
		pathQuery = fmt.Sprintf("$.keys[*] ? (@.%s >= %v)", property, value)
	case "$lt":
		pathQuery = fmt.Sprintf("$.keys[*] ? (@.%s < %v)", property, value)
	case "$le":
		pathQuery = fmt.Sprintf("$.keys[*] ? (@.%s <= %v)", property, value)
	default:
		return nil, fmt.Errorf("unsupported operation for array comparison: %s", operation)
	}

	// Return JSONB path exists query: semantic_id @? '$.keys[*] ? (@.value == "x")'
	// We need to cast the path string to JSONPATH type for PostgreSQL
	// Use L() with the complete SQL string (no placeholders) since pathQuery is already a complete string
	return goqu.L(fmt.Sprintf("semantic_id @? '%s'::jsonpath", pathQuery)), nil
}

// buildSubmodelElementJSONBQuery builds a JSONB path query for submodel element fields.
//
// Submodel elements are stored in the submodel_elements JSONB column as an array.
// This function generates queries to search within that array.
//
// Supported fields:
//   - $sme#semanticId -> shorthand for semanticId.keys[0].value
//   - $sme#semanticId.keys[N].value -> specific key index
//   - $sme#semanticId.keys[].value -> wildcard (any key)
//
// Parameters:
//   - field: The submodel element field reference (e.g., "$sme#semanticId.keys[1].value")
//   - value: The value to compare against
//   - operation: The comparison operator
//
// Returns:
//   - exp.Expression: A JSONB path query expression
//   - error: An error if the field format is invalid
func buildSubmodelElementJSONBQuery(field string, value interface{}, operation string) (exp.Expression, error) {
	// Parse the field to determine what we're querying
	var pathQuery string

	if field == "$sme#semanticId" {
		// Shorthand for semanticId.keys[0].value
		pathQuery = buildJSONBPathQuery("$[*].semanticId.keys[0].value", value, operation)
	} else if strings.Contains(field, ".keys[") {
	} else if field == "$sme#idShort" {
		// Direct idShort access
		pathQuery = buildJSONBPathQuery("$[*].idShort", value, operation)
	} else {
		return nil, fmt.Errorf("unsupported submodel element field: %s", field)
	}

	// Return JSONB path exists query
	return goqu.L(fmt.Sprintf("submodel_elements @? '%s'", pathQuery)), nil
}

// buildJSONBPathQuery constructs a JSONB path query string with the appropriate operator.
//
// Parameters:
//   - basePath: The base JSONB path (e.g., "$[*].semanticId.keys[0].value")
//   - value: The value to compare against
//   - operation: The comparison operator
//
// Returns:
//   - string: The complete JSONB path query
func buildJSONBPathQuery(basePath string, value interface{}, operation string) string {
	// For string values, we need to quote them
	valueStr := fmt.Sprintf("%v", value)
	if _, ok := value.(string); ok {
		valueStr = fmt.Sprintf("\"%s\"", value)
	}

	switch operation {
	case "$eq":
		return fmt.Sprintf("%s ? (@ == %s)", basePath, valueStr)
	case "$ne":
		return fmt.Sprintf("%s ? (@ != %s)", basePath, valueStr)
	case "$gt":
		return fmt.Sprintf("%s ? (@ > %s)", basePath, valueStr)
	case "$ge":
		return fmt.Sprintf("%s ? (@ >= %s)", basePath, valueStr)
	case "$lt":
		return fmt.Sprintf("%s ? (@ < %s)", basePath, valueStr)
	case "$le":
		return fmt.Sprintf("%s ? (@ <= %s)", basePath, valueStr)
	default:
		return fmt.Sprintf("%s ? (@ == %s)", basePath, valueStr) // Default to equality
	}
}

// toJSONBSQLComponent converts a Value operand to a SQL component using JSONB paths.
//
// This function maps field references to their JSONB path expressions for PostgreSQL
// queries, and wraps literal values appropriately.
//
// Parameters:
//   - operand: The Value to convert
//   - position: Description of operand position (for error messages)
//
// Returns:
//   - interface{}: Either a goqu identifier/literal for the JSONB path or a value
//   - error: An error if the operand is invalid
func toJSONBSQLComponent(operand *Value, position string) (interface{}, error) {
	if operand.IsField() {
		if operand.Field == nil {
			return nil, fmt.Errorf("%s operand is not a valid field", position)
		}
		fieldName := string(*operand.Field)
		jsonbPath, _, isJSONB := ParseAASQLFieldToJSONBPath(fieldName)

		if !isJSONB {
			// Regular column access
			return goqu.I(jsonbPath), nil
		}

		// JSONB path access - return as literal expression (not goqu.L which gets treated as a placeholder)
		return jsonbPath, nil
	}
	return operand.GetValue(), nil
}

// buildComparisonExpression is a helper function to build comparison expressions
func buildComparisonExpression(left interface{}, right interface{}, operation string) (exp.Expression, error) {
	// Check if left operand is a JSONB path string
	leftStr, leftIsString := left.(string)
	rightStr, rightIsString := right.(string)

	// Handle JSONB paths (strings that contain JSONB operators)
	leftIsJSONBPath := leftIsString && (strings.Contains(leftStr, "->") || strings.Contains(leftStr, "@?"))
	rightIsJSONBPath := rightIsString && (strings.Contains(rightStr, "->") || strings.Contains(rightStr, "@?"))

	var leftExpr, rightExpr interface{}

	if leftIsJSONBPath {
		// Convert JSONB path string to literal expression
		leftExpr = goqu.L(leftStr)
	} else if lit, ok := left.(exp.LiteralExpression); ok {
		leftExpr = lit
	} else if ident, ok := left.(exp.IdentifierExpression); ok {
		leftExpr = ident
	} else {
		// It's a value
		leftExpr = goqu.V(left)
	}

	if rightIsJSONBPath {
		// Convert JSONB path string to literal expression
		rightExpr = goqu.L(rightStr)
	} else if lit, ok := right.(exp.LiteralExpression); ok {
		rightExpr = lit
	} else if ident, ok := right.(exp.IdentifierExpression); ok {
		rightExpr = ident
	} else {
		// It's a value
		rightExpr = goqu.V(right)
	}

	switch operation {
	case "$eq":
		return goqu.L("? = ?", leftExpr, rightExpr), nil
	case "$ne":
		return goqu.L("? != ?", leftExpr, rightExpr), nil
	case "$gt":
		return goqu.L("? > ?", leftExpr, rightExpr), nil
	case "$ge":
		return goqu.L("? >= ?", leftExpr, rightExpr), nil
	case "$lt":
		return goqu.L("? < ?", leftExpr, rightExpr), nil
	case "$le":
		return goqu.L("? <= ?", leftExpr, rightExpr), nil
	default:
		return nil, fmt.Errorf("unsupported comparison operation: %s", operation)
	}
}
