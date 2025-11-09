# GORM/JSONB Query Guide for AAS Logical Expressions

## Overview

This guide explains how to use the `LogicalExpression` structure with GORM to query AAS Submodels stored with JSONB columns in PostgreSQL. The implementation supports type-safe query building using the goqu library, which generates PostgreSQL-specific JSONB queries.

## Architecture Changes

### From Normalized SQL to JSONB

**Old Approach (Normalized SQL):**
- Data stored across multiple tables: `submodel`, `reference`, `reference_key`, etc.
- Queries used JOINs and position constraints
- Example: `semantic_id_reference_key.value` with `position = 0`

**New Approach (GORM with JSONB):**
- Data stored in JSONB columns directly in the `submodel` table
- Queries use PostgreSQL JSONB operators (`->`, `->>`, `@?`)
- Example: `semantic_id->'keys'->0->>'value'`

### Key JSONB Operators

- `->` : Get JSON object field (returns JSON)
- `->>` : Get JSON object field as text (returns text)
- `@?` : Check if JSON path query returns any items
- `@>` : Check if JSON contains another JSON

## Field Mappings

The `ParseAASQLFieldToJSONBPath` function maps AAS query language fields to JSONB paths:

| AAS Field | JSONB Path | Description |
|-----------|------------|-------------|
| `$sm#idShort` | `id_short` | Regular column (not JSONB) |
| `$sm#id` | `submodel_id` | Regular column (not JSONB) |
| `$sm#semanticId` | `semantic_id->'keys'->0->>'value'` | Shorthand for first key's value |
| `$sm#semanticId.type` | `semantic_id->>'type'` | Reference type field |
| `$sm#semanticId.keys[].value` | `semantic_id->'keys'` | Array wildcard query |
| `$sm#semanticId.keys[N].value` | `semantic_id->'keys'->N->>'value'` | Specific array index |
| `$sm#semanticId.keys[N].type` | `semantic_id->'keys'->N->>'type'` | Specific key type |

## Usage Examples

### 1. Simple Equality Query

Query submodels by idShort:

```go
field := grammar.ModelStringPattern("$sm#idShort")
value := grammar.StandardString("exampleIdShort")

expr := grammar.LogicalExpression{
    Eq: []grammar.Value{
        {Field: &field},
        {StrVal: &value},
    },
}

goquExpr, err := expr.EvaluateToExpression()
if err != nil {
    panic(err)
}

// Use with GORM
count, err := gorm.G[model.Submodel](db).Where(goquExpr).Count(ctx, "*")
```

**Generated SQL:** `WHERE id_short = 'exampleIdShort'`

### 2. Query by SemanticId (Shorthand)

The shorthand `$sm#semanticId` automatically queries `keys[0].value`:

```go
field := grammar.ModelStringPattern("$sm#semanticId")
value := grammar.StandardString("https://example.com/semantic")

expr := grammar.LogicalExpression{
    Eq: []grammar.Value{
        {Field: &field},
        {StrVal: &value},
    },
}

goquExpr, err := expr.EvaluateToExpression()
```

**Generated SQL:** `WHERE semantic_id->'keys'->0->>'value' = 'https://example.com/semantic'`

### 3. Query Specific Key Index

Query a specific position in the keys array:

```go
field := grammar.ModelStringPattern("$sm#semanticId.keys[2].value")
value := grammar.StandardString("Submodel")

expr := grammar.LogicalExpression{
    Eq: []grammar.Value{
        {Field: &field},
        {StrVal: &value},
    },
}
```

**Generated SQL:** `WHERE semantic_id->'keys'->2->>'value' = 'Submodel'`

### 4. Array Wildcard Query

Match any element in the keys array:

```go
field := grammar.ModelStringPattern("$sm#semanticId.keys[].value")
value := grammar.StandardString("https://example.com/semantic")

expr := grammar.LogicalExpression{
    Eq: []grammar.Value{
        {Field: &field},
        {StrVal: &value},
    },
}
```

**Generated SQL:** `WHERE semantic_id @? '$.keys[*] ? (@.value == "https://example.com/semantic")'`

### 5. Complex AND Condition

Combine multiple conditions:

```go
idShortField := grammar.ModelStringPattern("$sm#idShort")
idShortValue := grammar.StandardString("exampleIdShort")
semanticField := grammar.ModelStringPattern("$sm#semanticId")
semanticValue := grammar.StandardString("semantic")

expr := grammar.LogicalExpression{
    And: []grammar.LogicalExpression{
        {
            Eq: []grammar.Value{
                {Field: &idShortField},
                {StrVal: &idShortValue},
            },
        },
        {
            Eq: []grammar.Value{
                {Field: &semanticField},
                {StrVal: &semanticValue},
            },
        },
    },
}
```

**Generated SQL:** `WHERE (id_short = 'exampleIdShort' AND semantic_id->'keys'->0->>'value' = 'semantic')`

### 6. OR Condition

```go
expr := grammar.LogicalExpression{
    Or: []grammar.LogicalExpression{
        {
            Eq: []grammar.Value{
                {Field: &field1},
                {StrVal: &value1},
            },
        },
        {
            Eq: []grammar.Value{
                {Field: &field2},
                {StrVal: &value2},
            },
        },
    },
}
```

### 7. NOT Condition

```go
expr := grammar.LogicalExpression{
    Not: &grammar.LogicalExpression{
        Eq: []grammar.Value{
            {Field: &field},
            {StrVal: &value},
        },
    },
}
```

### 8. Comparison Operators

All comparison operators are supported:

```go
// Equality
Eq: []grammar.Value{...}  // =

// Inequality  
Ne: []grammar.Value{...}  // !=

// Greater than
Gt: []grammar.Value{...}  // >

// Greater than or equal
Ge: []grammar.Value{...}  // >=

// Less than
Lt: []grammar.Value{...}  // <

// Less than or equal
Le: []grammar.Value{...}  // <=
```

## JSONB Structure in Database

The GORM models store nested structures as JSONB. For example:

```json
{
  "type": "ExternalReference",
  "keys": [
    {
      "type": "Submodel",
      "value": "https://example.com/semantic"
    },
    {
      "type": "GlobalReference", 
      "value": "additional-reference"
    }
  ],
  "referredSemanticId": {
    "type": "ModelReference",
    "keys": [...]
  }
}
```

## Performance Considerations

1. **JSONB Indexes**: Create GIN indexes on JSONB columns for better query performance:
   ```sql
   CREATE INDEX idx_submodel_semantic_id ON submodel USING GIN (semantic_id);
   ```

2. **Path Indexes**: For specific paths you query frequently:
   ```sql
   CREATE INDEX idx_semantic_id_keys ON submodel USING GIN ((semantic_id->'keys'));
   ```

3. **Array Wildcards**: Queries with `[]` wildcards use JSONB path queries which may be slower than direct index access.

4. **Specific Indexes**: When querying specific array positions frequently, consider extracting those to regular columns.

## Testing

Run the JSONB-specific tests:

```bash
go test ./internal/common/model/grammar/... -run JSONB
```

## Migration from Old SQL Approach

To migrate existing code:

1. **Update field references**: Change from table.column format to AAS field references
2. **Remove position constraints**: These are now handled automatically
3. **Update comparison logic**: Use the new `HandleComparison` function which supports JSONB paths
4. **Test thoroughly**: Ensure query results match between old and new implementations

## Troubleshooting

### Query not matching expected results

Check the generated SQL by examining the goqu expression. The JSONB operators are case-sensitive and whitespace-sensitive.

### Performance issues

Ensure appropriate JSONB indexes are created. Consider using specific array indexes instead of wildcards for frequently-queried positions.

### Type mismatches

Ensure you're using `->>` (text extraction) for string comparisons and `->` (JSON extraction) when you need to traverse deeper into the structure.

## Further Reading

- [PostgreSQL JSONB Documentation](https://www.postgresql.org/docs/current/datatype-json.html)
- [JSONB Indexing](https://www.postgresql.org/docs/current/datatype-json.html#JSON-INDEXING)
- [goqu Documentation](https://github.com/doug-martin/goqu)
- [GORM Documentation](https://gorm.io/docs/)
