# BaSyx SQL Schema In-Depth Documentation

This document provides a detailed explanation of the BaSyx SQL schema, including table purposes, field descriptions, relationships, and design rationale. It is intended for developers and database administrators who need to understand or extend the schema.

---

## Table of Contents
- [Extensions](#extensions)
- [Enums](#enums)
- [Core Tables](#core-tables)
  - [reference](#reference)
  - [reference_key](#reference_key)
  - [submodel](#submodel)
  - [submodel_element](#submodel_element)
- [Specialized Submodel Element Tables](#specialized-submodel-element-tables)
- [Collections and Lists](#collections-and-lists)
- [Entity, Operation, and Event Tables](#entity-operation-and-event-tables)
- [Qualifiers](#qualifiers)
- [Indexes and Performance](#indexes-and-performance)
- [Design Rationale](#design-rationale)

---

## Extensions

The schema uses PostgreSQL extensions for advanced features:
- `ltree`: Enables hierarchical path queries (used in `submodel_element.path_ltree`).
- `pg_trgm`: Provides fast trigram-based text search (used for text fields).

## Enums

Several enums are defined for strong typing and validation, e.g.:
- `modelling_kind`: 'Instance', 'Template'
- `aas_submodel_elements`: All AAS element types
- `data_type_def_xsd`: XSD-compatible data types
- ...and others (see [enums.md](./enums.md))

## Core Tables

### reference
Stores references (semantic IDs, etc.) as defined by the AAS standard.
- `id`: Primary key
- `type`: Enum (`reference_types`)

### reference_key
Stores ordered keys for a reference (to support multi-key references).
- `reference_id`: Foreign key to `reference`
- `position`: Array index (order matters)
- `type`, `value`: Key type and value

### submodel
Represents an AAS submodel.
- `id`: Primary key (AAS identifier)
- `id_short`, `category`, `kind`, `semantic_id`, `model_type`: Metadata fields

### submodel_element
Represents any submodel element (tree node).
- `id`: Primary key
- `submodel_id`: Foreign key to `submodel`
- `parent_sme_id`: Parent element (for tree structure)
- `position`: Order among siblings (for lists/collections)
- `id_short`, `category`, `model_type`, `semantic_id`, `path_ltree`: Metadata and hierarchy
- **Tree Structure**: Elements are organized as a tree using `parent_sme_id` and `path_ltree` (ltree path for fast queries).

## Specialized Submodel Element Tables

Each AAS element type with additional data has its own table, always with a 1:1 relationship to `submodel_element`:
- `property_element`: Stores typed property values (text, numeric, boolean, time, datetime, reference)
- `multilanguage_property`, `multilanguage_property_value`: Multi-language support
- `blob_element`, `file_element`: Binary and file data
- `range_element`: Min/max values for ranges
- `reference_element`: Reference values
- `relationship_element`, `annotated_rel_annotation`: Relationships and annotations

## Collections and Lists

- `submodel_element_collection`: Marker table for collections
- `submodel_element_list`: Stores list-specific metadata (order relevance, element type, value type, semantic ID for list elements)

## Entity, Operation, and Event Tables

- `entity_element`, `entity_specific_asset_id`: Entity type and asset IDs
- `operation_element`, `operation_variable`: Operations and their variables (in/out/inout)
- `basic_event_element`: Event elements with state, direction, and timing
- `capability_element`: Marker for capability elements

## Qualifiers

- `qualifier`: Qualifiers can be attached to any submodel element, with typed value fields and kind/type metadata

## Indexes and Performance

- **GIN/Trigram Indexes**: For fast text search (e.g., `value_text`, `key_value`)
- **GIST/Ltree Indexes**: For efficient tree/hierarchy queries (e.g., `path_ltree`)
- **Partial Indexes**: On value columns, filtered by type (e.g., numeric, date, boolean)
- **Unique Constraints**: Ensure unique `id_short` and `position` among siblings

## Design Rationale

- **Normalization**: References and semantic keys are normalized for reuse and efficient lookup.
- **Tree Structure**: All submodel elements are stored in a single table with a parent-child relationship, supporting arbitrary nesting and fast path queries.
- **Specialization**: Each AAS element type with extra data has its own table, linked 1:1 to `submodel_element`.
- **Extensibility**: The schema is designed to be extensible for future AAS versions and custom elements.
- **Performance**: Indexes and partial indexes are used to optimize common queries, especially for text and hierarchy.

---

For a full list of tables and fields, see [entities.md](./entities.md). For diagrams, see [relationships.md](./relationships.md).

For questions about the AAS standard, refer to the IDTA documentation. BaSyx implements this standard as a server platform.
