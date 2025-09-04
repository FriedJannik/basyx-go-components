# Indexes & Performance

The schema uses several indexes to optimize query performance, especially for text search and hierarchical queries.

## Key Indexes
- **GIN/Trigram Indexes**: For fast text search on fields like `value_text`, `key_value`, etc.
- **GIST/Ltree Indexes**: For efficient hierarchical path queries on `path_ltree`.
- **Partial Indexes**: On value columns, filtered by type (e.g., numeric, date, boolean).

## Example Index Definitions
```sql
CREATE INDEX ix_prop_text_trgm ON property_element USING GIN (value_text gin_trgm_ops) WHERE value_type = 'xs:string';
CREATE INDEX ix_sme_path_gist ON submodel_element USING GIST (path_ltree);
```

For more details, see the schema file.
