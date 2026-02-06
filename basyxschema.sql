/*******************************************************************************
* Copyright (C) 2026 the Eclipse BaSyx Authors and Fraunhofer IESE
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

-- ------------------------------------------
-- Extensions
-- ------------------------------------------
CREATE EXTENSION IF NOT EXISTS ltree;
CREATE EXTENSION IF NOT EXISTS pg_trgm;


-- ------------------------------------------
-- Enums
-- ------------------------------------------
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'security_type') THEN
    CREATE TYPE security_type AS ENUM ('NONE', 'RFC_TLSA', 'W3C_DID');
  END IF;
END $$;

-- ------------------------------------------
-- Tables
-- ------------------------------------------
CREATE TABLE IF NOT EXISTS reference (
  id           BIGSERIAL PRIMARY KEY,
  type         int NOT NULL,
  parentReference BIGINT REFERENCES reference(id),  -- Optional nesting
  rootReference BIGINT REFERENCES reference(id)  -- The root of the nesting tree
);
CREATE TABLE IF NOT EXISTS reference_key (
  id           BIGSERIAL PRIMARY KEY,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE,
  position     INTEGER NOT NULL,                -- <- Array-Index keys[i]
  type         int    NOT NULL,
  value        TEXT     NOT NULL,
  UNIQUE(reference_id, position)
);

CREATE TABLE IF NOT EXISTS lang_string_text_type_reference(
  id       BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY
);
CREATE TABLE IF NOT EXISTS lang_string_text_type (
  id     BIGSERIAL PRIMARY KEY,
  lang_string_text_type_reference_id BIGINT NOT NULL REFERENCES lang_string_text_type_reference(id) ON DELETE CASCADE,
  language TEXT NOT NULL,
  text     varchar(1023) NOT NULL
);
CREATE TABLE IF NOT EXISTS lang_string_name_type_reference(
  id       BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY
);
CREATE TABLE IF NOT EXISTS lang_string_name_type (
  id     BIGSERIAL PRIMARY KEY,
  lang_string_name_type_reference_id BIGINT NOT NULL REFERENCES lang_string_name_type_reference(id) ON DELETE CASCADE,
  language TEXT NOT NULL,
  text     varchar(128) NOT NULL
);
CREATE TABLE IF NOT EXISTS administrative_information (
  id                BIGSERIAL PRIMARY KEY,
  version           VARCHAR(4),
  revision          VARCHAR(4),
  creator           BIGINT REFERENCES reference(id),
  embedded_data_specification JSONB,
  templateId        VARCHAR(2048)
);
CREATE TABLE IF NOT EXISTS data_specification_content (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY
);
CREATE TABLE IF NOT EXISTS data_specification (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  data_specification BIGINT REFERENCES reference(id) NOT NULL,
  data_specification_content BIGINT REFERENCES data_specification_content(id) NOT NULL
);
CREATE TABLE IF NOT EXISTS value_list (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY
);
CREATE TABLE IF NOT EXISTS value_list_value_reference_pair (
  id BIGSERIAL PRIMARY KEY,
  position INTEGER NOT NULL,  -- <- Array-Index valueReferencePairs[i]
  value_list_id BIGINT NOT NULL REFERENCES value_list(id) ON DELETE CASCADE,
  value TEXT NOT NULL,
  value_id BIGINT REFERENCES reference(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS level_type (
  id BIGSERIAL PRIMARY KEY,
  min BOOLEAN NOT NULL,
  max BOOLEAN NOT NULL,
  nom BOOLEAN NOT NULL,
  typ BOOLEAN NOT NULL
);
CREATE TABLE IF NOT EXISTS data_specification_iec61360 (
  id                BIGINT REFERENCES data_specification_content(id) ON DELETE CASCADE PRIMARY KEY,
  position          INTEGER,
  preferred_name_id BIGINT REFERENCES lang_string_text_type_reference(id) ON DELETE CASCADE NOT NULL,
  short_name_id     BIGINT REFERENCES lang_string_text_type_reference(id) ON DELETE CASCADE,
  unit              TEXT,
  unit_id           BIGINT REFERENCES reference(id) ON DELETE CASCADE,
  source_of_definition TEXT,
  symbol           TEXT,
  data_type        int,
  definition_id    BIGINT REFERENCES lang_string_text_type_reference(id) ON DELETE CASCADE,
  value_format     TEXT,
  value_list_id    BIGINT REFERENCES value_list(id) ON DELETE CASCADE,
  level_type_id BIGINT REFERENCES level_type(id) ON DELETE CASCADE,
  value VARCHAR(2048)
);
CREATE TABLE IF NOT EXISTS administrative_information_embedded_data_specification (
  id                BIGSERIAL PRIMARY KEY,
  administrative_information_id BIGINT REFERENCES administrative_information(id) ON DELETE CASCADE,
  embedded_data_specification_id BIGSERIAL REFERENCES data_specification(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS submodel (
  id          varchar(2048) PRIMARY KEY,                 -- Identifiable.id
  id_short    varchar(128),
  category    varchar(128),
  kind        int,
  embedded_data_specification JSONB DEFAULT '[]',
  supplemental_semantic_ids JSONB DEFAULT '[]',
  extensions JSONB DEFAULT '[]',
  administration_id BIGINT REFERENCES administrative_information(id) ON DELETE CASCADE,
  semantic_id BIGINT REFERENCES reference(id) ON DELETE CASCADE,
  description_id BIGINT REFERENCES lang_string_text_type_reference(id) ON DELETE CASCADE,
  displayname_id  BIGINT REFERENCES lang_string_name_type_reference(id) ON DELETE CASCADE,
  model_type  int NOT NULL DEFAULT 7
);
CREATE TABLE IF NOT EXISTS submodel_supplemental_semantic_id (
  id BIGSERIAL PRIMARY KEY,
  submodel_id VARCHAR(2048) NOT NULL REFERENCES submodel(id) ON DELETE CASCADE,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS extension (
  id          BIGSERIAL PRIMARY KEY,
  semantic_id BIGINT REFERENCES reference(id) ON DELETE CASCADE,
  name       varchar(128) NOT NULL,
  position   INTEGER,
  value_type    int,
  value_text    TEXT,
  value_num     NUMERIC,
  value_bool    BOOLEAN,
  value_time    TIME,
  value_date    DATE,
  value_datetime TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS submodel_extension (
  id BIGSERIAL PRIMARY KEY,
  submodel_id VARCHAR(2048) NOT NULL REFERENCES submodel(id) ON DELETE CASCADE,
  extension_id BIGINT NOT NULL REFERENCES extension(id) ON DELETE CASCADE 
);
CREATE TABLE IF NOT EXISTS extension_supplemental_semantic_id (
  id BIGSERIAL PRIMARY KEY,
  extension_id BIGINT NOT NULL REFERENCES extension(id) ON DELETE CASCADE,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE
); 
CREATE TABLE IF NOT EXISTS extension_refers_to (
  id BIGSERIAL PRIMARY KEY,
  extension_id BIGINT NOT NULL REFERENCES extension(id) ON DELETE CASCADE,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS submodel_element (
  id             BIGSERIAL PRIMARY KEY,
  submodel_id    TEXT NOT NULL REFERENCES submodel(id) ON DELETE CASCADE,
  root_sme_id  BIGINT REFERENCES submodel_element(id) ON DELETE CASCADE,
  parent_sme_id  BIGINT REFERENCES submodel_element(id) ON DELETE CASCADE,
  position       INTEGER,                                   -- for ordering in lists
  id_short       varchar(128),
  category       varchar(128),
  model_type     int NOT NULL,
  embedded_data_specification JSONB DEFAULT '[]',
  supplemental_semantic_ids JSONB DEFAULT '[]',
  extensions JSONB DEFAULT '[]',
  semantic_id    BIGINT REFERENCES reference(id),
  description_id BIGINT REFERENCES lang_string_text_type_reference(id) ON DELETE CASCADE,
  displayname_id BIGINT REFERENCES lang_string_name_type_reference(id) ON DELETE CASCADE,
  idshort_path   TEXT NOT NULL,                            -- e.g. sm_abc.sensors[2].temperature
  depth	BIGINT,
  CONSTRAINT uq_sibling_idshort UNIQUE (submodel_id, parent_sme_id, idshort_path),
  CONSTRAINT uq_sibling_pos     UNIQUE (submodel_id, parent_sme_id, position)
);
CREATE TABLE IF NOT EXISTS submodel_element_supplemental_semantic_id (
  submodel_element_id       BIGINT NOT NULL REFERENCES submodel_element(id) ON DELETE CASCADE,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE,
  PRIMARY KEY (submodel_element_id, reference_id)
);
CREATE TABLE IF NOT EXISTS submodel_element_extension (
  submodel_element_id       BIGINT NOT NULL REFERENCES submodel_element(id) ON DELETE CASCADE,
  extension_id BIGINT NOT NULL REFERENCES extension(id) ON DELETE CASCADE,
  PRIMARY KEY (submodel_element_id, extension_id)
);
CREATE TABLE IF NOT EXISTS submodel_element_embedded_data_specification (
  submodel_element_id BIGINT REFERENCES submodel_element(id) ON DELETE CASCADE,
  embedded_data_specification_id BIGSERIAL REFERENCES data_specification(id) ON DELETE CASCADE,
  PRIMARY KEY (submodel_element_id, embedded_data_specification_id)
);
CREATE TABLE IF NOT EXISTS property_element (
  id            BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  value_type    int NOT NULL,
  value_text    TEXT,
  value_num     NUMERIC,
  value_bool    BOOLEAN,
  value_time    TIME,
  value_date    DATE,
  value_datetime TIMESTAMPTZ,
  value_id      BIGINT REFERENCES reference(id)
);
CREATE TABLE IF NOT EXISTS multilanguage_property (
  id        BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  value_id  BIGINT REFERENCES reference(id)
);
CREATE TABLE IF NOT EXISTS multilanguage_property_value (
  id     BIGSERIAL PRIMARY KEY,
  mlp_id BIGINT NOT NULL REFERENCES multilanguage_property(id) ON DELETE CASCADE,
  language TEXT NOT NULL,
  text     TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS blob_element (
  id           BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  content_type TEXT,
  value        BYTEA
);

CREATE TABLE IF NOT EXISTS file_element (
  id           BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  content_type TEXT,
  file_name    TEXT,
  value        TEXT
);
CREATE TABLE IF NOT EXISTS file_data (
  id BIGINT PRIMARY KEY REFERENCES file_element(id) ON DELETE CASCADE,
  file_oid oid
);
CREATE TABLE IF NOT EXISTS range_element (
  id            BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  value_type    int NOT NULL,
  min_text      TEXT,  max_text      TEXT,
  min_num       NUMERIC, max_num     NUMERIC,
  min_time      TIME,   max_time     TIME,
  min_date      DATE,   max_date     DATE,
  min_datetime  TIMESTAMPTZ, max_datetime TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS reference_element (
  id        BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  value JSONB
);
CREATE TABLE IF NOT EXISTS relationship_element (
  id         BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  first JSONB,
  second JSONB
);
CREATE TABLE IF NOT EXISTS annotated_relationship_element (
  id         BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  first JSONB,
  second JSONB
);
CREATE TABLE IF NOT EXISTS submodel_element_collection (
  id BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS submodel_element_list (
  id                         BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  order_relevant             BOOLEAN,
  semantic_id_list_element   JSONB,
  type_value_list_element    int NOT NULL,
  value_type_list_element    int
);
CREATE TABLE IF NOT EXISTS entity_element (
  id              BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  entity_type     int NOT NULL,
  global_asset_id TEXT,
  specific_asset_ids JSONB DEFAULT '[]'
);
CREATE TABLE IF NOT EXISTS entity_specific_asset_id (
  id                   BIGSERIAL PRIMARY KEY,
  entity_id            BIGINT NOT NULL REFERENCES entity_element(id) ON DELETE CASCADE,
  name                 TEXT NOT NULL,
  value                TEXT NOT NULL,
  external_subject_ref BIGINT REFERENCES reference(id)
);
CREATE TABLE IF NOT EXISTS operation_element (
  id BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  input_variables JSONB DEFAULT '[]',
  output_variables JSONB DEFAULT '[]',
  inoutput_variables JSONB DEFAULT '[]'
);
CREATE TABLE IF NOT EXISTS operation_variable (
  id           BIGSERIAL PRIMARY KEY,
  operation_id BIGINT NOT NULL REFERENCES operation_element(id) ON DELETE CASCADE,
  role         int NOT NULL,
  position     INTEGER NOT NULL,
  value_sme    BIGINT NOT NULL REFERENCES submodel_element(id) ON DELETE CASCADE,
  UNIQUE (operation_id, role, position)
);
CREATE TABLE IF NOT EXISTS basic_event_element (
  id                BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE,
  observed          JSONB,
  direction         int NOT NULL,
  state             int NOT NULL,
  message_topic     TEXT,
  message_broker    JSONB,
  last_update       TIMESTAMPTZ,
  min_interval      INTERVAL,
  max_interval      INTERVAL
);
CREATE TABLE IF NOT EXISTS capability_element (
  id BIGINT PRIMARY KEY REFERENCES submodel_element(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS qualifier (
  id                BIGSERIAL PRIMARY KEY,
  position          INTEGER NOT NULL,
  kind              int,
  type              TEXT NOT NULL,
  value_type        int NOT NULL,
  value_text        TEXT,
  value_num         NUMERIC,
  value_bool        BOOLEAN,
  value_time        TIME,
  value_date        DATE,
  value_datetime    TIMESTAMPTZ,
  value_id          BIGINT REFERENCES reference(id),
  semantic_id       BIGINT REFERENCES reference(id)
);
CREATE TABLE IF NOT EXISTS submodel_element_qualifier (
  sme_id      BIGINT NOT NULL REFERENCES submodel_element(id) ON DELETE CASCADE,
  qualifier_id BIGINT NOT NULL REFERENCES qualifier(id) ON DELETE CASCADE,
  PRIMARY KEY (sme_id, qualifier_id)
);
CREATE TABLE IF NOT EXISTS submodel_qualifier (
  id BIGSERIAL PRIMARY KEY,
  submodel_id  VARCHAR(2048) NOT NULL REFERENCES submodel(id) ON DELETE CASCADE,
  qualifier_id BIGINT NOT NULL REFERENCES qualifier(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS qualifier_supplemental_semantic_id (
  id BIGSERIAL PRIMARY KEY,
  qualifier_id BIGINT NOT NULL REFERENCES qualifier(id) ON DELETE CASCADE,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS descriptor (
  id BIGSERIAL PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS descriptor_extension (
  id BIGSERIAL PRIMARY KEY,
  descriptor_id BIGINT NOT NULL REFERENCES descriptor(id) ON DELETE CASCADE,
  extension_id BIGINT NOT NULL REFERENCES extension(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS specific_asset_id (
  id BIGSERIAL PRIMARY KEY,
  position     INTEGER NOT NULL,                -- <- Array-Index
  descriptor_id BIGINT NOT NULL REFERENCES descriptor(id) ON DELETE CASCADE,
  semantic_id BIGINT REFERENCES reference(id),
  name VARCHAR(64) NOT NULL,
  value VARCHAR(2048) NOT NULL,
  external_subject_ref BIGINT REFERENCES reference(id)
);


CREATE TABLE IF NOT EXISTS specific_asset_id_supplemental_semantic_id (
  id BIGSERIAL PRIMARY KEY,
  specific_asset_id_id BIGINT NOT NULL REFERENCES specific_asset_id(id) ON DELETE CASCADE,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS aas_descriptor_endpoint (
  id BIGSERIAL PRIMARY KEY,
  descriptor_id BIGINT NOT NULL REFERENCES descriptor(id) ON DELETE CASCADE,
  position     INTEGER NOT NULL,                -- <- Array-Index
  href VARCHAR(2048) NOT NULL,
  endpoint_protocol VARCHAR(128),
  sub_protocol VARCHAR(128),
  sub_protocol_body VARCHAR(2048),
  sub_protocol_body_encoding VARCHAR(128),
  interface VARCHAR(128) NOT NULL
);

CREATE TABLE IF NOT EXISTS security_attributes (
  id BIGSERIAL NOT NULL PRIMARY KEY,
  endpoint_id BIGINT NOT NULL REFERENCES aas_descriptor_endpoint(id) ON DELETE CASCADE,
  security_type security_type NOT NULL,
  security_key TEXT NOT NULL,
  security_value TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS endpoint_protocol_version (
  id BIGSERIAL PRIMARY KEY,
  endpoint_id BIGINT NOT NULL REFERENCES aas_descriptor_endpoint(id) ON DELETE CASCADE,
  endpoint_protocol_version VARCHAR(128) NOT NULL
);


CREATE TABLE IF NOT EXISTS aas_descriptor (
  descriptor_id BIGINT PRIMARY KEY REFERENCES descriptor(id) ON DELETE CASCADE,
  description_id BIGINT REFERENCES lang_string_text_type_reference(id) ON DELETE SET NULL,
  displayname_id BIGINT REFERENCES lang_string_name_type_reference(id) ON DELETE SET NULL,
  administrative_information_id BIGINT REFERENCES administrative_information(id) ON DELETE CASCADE,
  asset_kind int,
  asset_type VARCHAR(2048),
  global_asset_id VARCHAR(2048),
  id_short VARCHAR(128),
  id VARCHAR(2048) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS submodel_descriptor (
  descriptor_id BIGINT PRIMARY KEY REFERENCES descriptor(id) ON DELETE CASCADE,
  position     INTEGER NOT NULL,                -- <- Array-Index
  aas_descriptor_id BIGINT REFERENCES aas_descriptor(descriptor_id) ON DELETE CASCADE,
  description_id BIGINT REFERENCES lang_string_text_type_reference(id) ON DELETE SET NULL,
  displayname_id BIGINT REFERENCES lang_string_name_type_reference(id) ON DELETE SET NULL,
  administrative_information_id BIGINT REFERENCES administrative_information(id) ON DELETE CASCADE,
  id_short VARCHAR(128),
  id VARCHAR(2048) NOT NULL, -- not unique because it can have duplicates over different aas descriptor.
  semantic_id BIGINT REFERENCES reference(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS submodel_descriptor_supplemental_semantic_id (
  id BIGSERIAL PRIMARY KEY,
  descriptor_id BIGINT NOT NULL REFERENCES submodel_descriptor(descriptor_id) ON DELETE CASCADE,
  reference_id BIGINT NOT NULL REFERENCES reference(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS submodel_embedded_data_specification (
  id                BIGSERIAL PRIMARY KEY,
  submodel_id       VARCHAR(2048) REFERENCES submodel(id) ON DELETE CASCADE,
  embedded_data_specification_id BIGSERIAL REFERENCES data_specification(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS registry_descriptor (
  descriptor_id BIGINT PRIMARY KEY REFERENCES descriptor(id) ON DELETE CASCADE,
  description_id BIGINT REFERENCES lang_string_text_type_reference(id),
  displayname_id BIGINT REFERENCES lang_string_name_type_reference(id),
  administrative_information_id BIGINT REFERENCES administrative_information(id),
  registry_type VARCHAR(2048),
  global_asset_id VARCHAR(2048),
  id_short VARCHAR(128),
  id VARCHAR(2048) NOT NULL UNIQUE,
  company VARCHAR(2048)
);
-- ------------------------------------------
-- Indexes
-- ------------------------------------------
-- Naming convention: ix_<table_abbrev>_<column(s)>
-- All indexes use IF NOT EXISTS to allow idempotent schema application.

-- ==========================================
-- Reference Tables
-- ==========================================
-- References are heavily joined when reconstructing AAS objects (semantic_id, value_id, etc.)

-- Speeds up recursive reference tree traversal when loading nested references
CREATE INDEX IF NOT EXISTS ix_ref_rootref ON reference(rootReference);

-- Filters references by type (ModelReference, ExternalReference) during reconstruction
CREATE INDEX IF NOT EXISTS ix_ref_type ON reference(type);

-- FK index: accelerates JOIN from reference_key to parent reference during key retrieval
CREATE INDEX IF NOT EXISTS ix_refkey_reference_id ON reference_key(reference_id);

-- Composite index for searching reference keys by type and value (e.g., finding semantic matches)
CREATE INDEX IF NOT EXISTS ix_refkey_type_val ON reference_key(type, value);

-- Trigram index enables fuzzy/partial text search on reference key values (LIKE '%pattern%')
CREATE INDEX IF NOT EXISTS ix_refkey_val_trgm ON reference_key USING GIN (value gin_trgm_ops);

-- ==========================================
-- Lang String Types
-- ==========================================
-- Lang strings store multilingual descriptions/display names; joined via reference tables

-- FK index: speeds up JOIN to retrieve all language entries for a text type reference
CREATE INDEX IF NOT EXISTS ix_lstt_refid ON lang_string_text_type(lang_string_text_type_reference_id);

-- FK index: speeds up JOIN to retrieve all language entries for a name type reference
CREATE INDEX IF NOT EXISTS ix_lsnt_refid ON lang_string_name_type(lang_string_name_type_reference_id);

-- ==========================================
-- Administrative Information
-- ==========================================
-- Administrative info includes version, revision, and creator reference

-- FK index: speeds up JOIN to resolve creator reference when loading admin info
CREATE INDEX IF NOT EXISTS ix_ai_creator ON administrative_information(creator);

-- ==========================================
-- Data Specification (IEC 61360)
-- ==========================================
-- Data specifications provide standardized metadata for submodel elements

-- FK index: speeds up JOIN from data_specification to its content record
CREATE INDEX IF NOT EXISTS ix_ds_dataspec_content ON data_specification(data_specification_content);

-- FK index: speeds up retrieval of value reference pairs belonging to a value list
CREATE INDEX IF NOT EXISTS ix_vlvrp_valuelist ON value_list_value_reference_pair(value_list_id);

-- FK index: speeds up JOIN to resolve value_id reference in value list pairs
CREATE INDEX IF NOT EXISTS ix_vlvrp_value_id ON value_list_value_reference_pair(value_id);

-- FK index: speeds up JOIN from IEC61360 spec to its value list
CREATE INDEX IF NOT EXISTS ix_iec61360_value_list_id ON data_specification_iec61360(value_list_id);

-- FK index: speeds up JOIN from IEC61360 spec to its level type
CREATE INDEX IF NOT EXISTS ix_iec61360_level_type_id ON data_specification_iec61360(level_type_id);

-- ==========================================
-- Submodel
-- ==========================================
-- Submodels are core AAS entities; frequently queried and joined with related tables

-- Enables filtering submodels by id_short (API query parameter)
CREATE INDEX IF NOT EXISTS ix_sm_idshort ON submodel(id_short);

-- FK index: speeds up JOIN to load administrative information for a submodel
CREATE INDEX IF NOT EXISTS ix_sm_admin_id ON submodel(administration_id);

-- FK index: speeds up JOIN to resolve semantic_id reference for submodel
CREATE INDEX IF NOT EXISTS ix_sm_semantic_id ON submodel(semantic_id);

-- FK index: speeds up JOIN to load description lang strings for submodel
CREATE INDEX IF NOT EXISTS ix_sm_desc_id ON submodel(description_id);

-- FK index: speeds up JOIN to load display name lang strings for submodel
CREATE INDEX IF NOT EXISTS ix_sm_displayname_id ON submodel(displayname_id);

-- FK index: speeds up retrieval of supplemental semantic IDs for a submodel
CREATE INDEX IF NOT EXISTS ix_smssi_submodel_id ON submodel_supplemental_semantic_id(submodel_id);

-- FK index: speeds up JOIN to resolve reference for supplemental semantic ID
CREATE INDEX IF NOT EXISTS ix_smssi_reference_id ON submodel_supplemental_semantic_id(reference_id);

-- FK index: speeds up retrieval of embedded data specifications for a submodel
CREATE INDEX IF NOT EXISTS ix_seds_submodel ON submodel_embedded_data_specification(submodel_id);

-- ==========================================
-- Extensions
-- ==========================================
-- Extensions add custom metadata to submodels and submodel elements

-- FK index: speeds up retrieval of all extensions for a submodel
CREATE INDEX IF NOT EXISTS ix_smext_submodel_id ON submodel_extension(submodel_id);

-- FK index: speeds up JOIN from junction table to extension record
CREATE INDEX IF NOT EXISTS ix_smext_extension_id ON submodel_extension(extension_id);

-- FK index: speeds up JOIN to resolve semantic_id reference for extension
CREATE INDEX IF NOT EXISTS ix_ext_semantic_id ON extension(semantic_id);

-- FK index: speeds up retrieval of supplemental semantic IDs for an extension
CREATE INDEX IF NOT EXISTS ix_essi_extension_id ON extension_supplemental_semantic_id(extension_id);

-- FK index: speeds up JOIN to resolve reference for extension supplemental semantic ID
CREATE INDEX IF NOT EXISTS ix_essi_reference_id ON extension_supplemental_semantic_id(reference_id);

-- FK index: speeds up retrieval of refers_to references for an extension
CREATE INDEX IF NOT EXISTS ix_extref_extension_id ON extension_refers_to(extension_id);

-- FK index: speeds up JOIN to resolve refers_to reference
CREATE INDEX IF NOT EXISTS ix_extref_reference_id ON extension_refers_to(reference_id);

-- ==========================================
-- Submodel Elements (Most Critical)
-- ==========================================
-- Submodel elements are the most frequently queried table; these indexes are essential

-- Composite index: primary lookup pattern - find element by submodel + idshort_path
CREATE INDEX IF NOT EXISTS ix_sme_sub_path ON submodel_element(submodel_id, idshort_path);

-- Composite index: speeds up ordered retrieval of child elements under a parent
CREATE INDEX IF NOT EXISTS ix_sme_parent_pos ON submodel_element(parent_sme_id, position);

-- Composite index: enables filtering elements by model_type within a submodel
CREATE INDEX IF NOT EXISTS ix_sme_sub_type ON submodel_element(submodel_id, model_type);

-- Composite index: speeds up hierarchical queries (find all children of a parent)
CREATE INDEX IF NOT EXISTS ix_sme_sub_parent ON submodel_element(submodel_id, parent_sme_id);

-- FK index: speeds up queries finding all elements under a root element (for cascade operations)
CREATE INDEX IF NOT EXISTS ix_sme_root_sme_id ON submodel_element(root_sme_id);

-- Trigram index: enables fuzzy/partial search on idshort_path (LIKE '%pattern%')
CREATE INDEX IF NOT EXISTS ix_sme_path_gin ON submodel_element USING GIN (idshort_path gin_trgm_ops);

-- FK index: speeds up retrieval of supplemental semantic IDs for a submodel element
CREATE INDEX IF NOT EXISTS ix_smessi_smeid ON submodel_element_supplemental_semantic_id(submodel_element_id);

-- FK index: speeds up retrieval of extensions for a submodel element
CREATE INDEX IF NOT EXISTS ix_smeext_smeid ON submodel_element_extension(submodel_element_id);

-- FK index: speeds up retrieval of embedded data specifications for a submodel element
CREATE INDEX IF NOT EXISTS ix_smeeds_smeid ON submodel_element_embedded_data_specification(submodel_element_id);

-- ==========================================
-- Property Elements
-- ==========================================
-- Property elements store typed values with optional value_id reference

-- FK index: speeds up JOIN to resolve value_id reference when loading properties
CREATE INDEX IF NOT EXISTS ix_property_value_id ON property_element(value_id);

-- ==========================================
-- MultiLanguage Property
-- ==========================================
-- MultiLanguage properties store text in multiple languages

-- Composite index: speeds up retrieval of specific language value for a MLP
CREATE INDEX IF NOT EXISTS ix_mlp_lang ON multilanguage_property_value(mlp_id, language);

-- Trigram index: enables fuzzy/partial text search across MLP values
CREATE INDEX IF NOT EXISTS ix_mlp_text_trgm ON multilanguage_property_value USING GIN (text gin_trgm_ops);

-- FK index: speeds up JOIN to resolve value_id reference for MLP
CREATE INDEX IF NOT EXISTS ix_mlp_value_id ON multilanguage_property(value_id);

-- ==========================================
-- File Element
-- ==========================================
-- File elements reference external files via URL/path

-- Trigram index: enables fuzzy/partial search on file paths/URLs
CREATE INDEX IF NOT EXISTS ix_file_value_trgm ON file_element USING GIN (value gin_trgm_ops);

-- ==========================================
-- Basic Event Element
-- ==========================================
-- Event elements track state changes with timestamps

-- Index on timestamp: enables efficient time-based queries (e.g., recent events)
CREATE INDEX IF NOT EXISTS ix_bee_lastupd ON basic_event_element(last_update);

-- ==========================================
-- Qualifiers
-- ==========================================
-- Qualifiers add constraints/metadata to submodels and submodel elements

-- FK index: speeds up JOIN to resolve semantic_id reference for qualifier
CREATE INDEX IF NOT EXISTS ix_qual_semantic_id ON qualifier(semantic_id);

-- FK index: speeds up JOIN to resolve value_id reference for qualifier
CREATE INDEX IF NOT EXISTS ix_qual_value_id ON qualifier(value_id);

-- Enables filtering qualifiers by type (e.g., "Multiplicity", "ExpressionSemantic")
CREATE INDEX IF NOT EXISTS ix_qual_type ON qualifier(type);

-- FK index: speeds up retrieval of all qualifiers for a submodel element
CREATE INDEX IF NOT EXISTS ix_qual_sme ON submodel_element_qualifier(sme_id);

-- FK index: speeds up retrieval of all qualifiers for a submodel
CREATE INDEX IF NOT EXISTS ix_smq_submodel_id ON submodel_qualifier(submodel_id);

-- FK index: speeds up JOIN from junction table to qualifier record
CREATE INDEX IF NOT EXISTS ix_smq_qualifier_id ON submodel_qualifier(qualifier_id);

-- FK index: speeds up retrieval of supplemental semantic IDs for a qualifier
CREATE INDEX IF NOT EXISTS ix_qssi_qualifier_id ON qualifier_supplemental_semantic_id(qualifier_id);

-- FK index: speeds up JOIN to resolve reference for qualifier supplemental semantic ID
CREATE INDEX IF NOT EXISTS ix_qssi_reference_id ON qualifier_supplemental_semantic_id(reference_id);

-- ==========================================
-- Descriptor Extensions
-- ==========================================
-- Extensions for AAS/Submodel descriptors in registry context

-- FK index: speeds up retrieval of all extensions for a descriptor
CREATE INDEX IF NOT EXISTS ix_descriptor_extension_descriptor_id ON descriptor_extension(descriptor_id);

-- FK index: speeds up JOIN from junction table to extension record
CREATE INDEX IF NOT EXISTS ix_descriptor_extension_extension_id ON descriptor_extension(extension_id);

-- ==========================================
-- Specific Asset IDs
-- ==========================================
-- Specific asset IDs enable discovery of AAS by asset identifiers

-- FK index: speeds up retrieval of all specific asset IDs for a descriptor
CREATE INDEX IF NOT EXISTS ix_specasset_descriptor_id ON specific_asset_id(descriptor_id);

-- FK index: speeds up JOIN to resolve semantic_id reference
CREATE INDEX IF NOT EXISTS ix_specasset_semantic_id ON specific_asset_id(semantic_id);

-- Enables filtering by asset ID name (e.g., "serialNumber", "batchId")
CREATE INDEX IF NOT EXISTS ix_specasset_name ON specific_asset_id(name);

-- Composite index: primary discovery pattern - find by name AND value
CREATE INDEX IF NOT EXISTS ix_specasset_name_value ON specific_asset_id(name, value);

-- Trigram index: enables fuzzy/partial search on asset ID values
CREATE INDEX IF NOT EXISTS ix_specasset_value_trgm ON specific_asset_id USING GIN (value gin_trgm_ops);

-- FK index: speeds up retrieval of supplemental semantic IDs for specific asset ID
CREATE INDEX IF NOT EXISTS ix_specasset_supp_spec_id ON specific_asset_id_supplemental_semantic_id(specific_asset_id_id);

-- FK index: speeds up JOIN to resolve reference for supplemental semantic ID
CREATE INDEX IF NOT EXISTS ix_specasset_supp_ref_id ON specific_asset_id_supplemental_semantic_id(reference_id);

-- ==========================================
-- AAS Descriptor Endpoints
-- ==========================================
-- Endpoints define how to access AAS/Submodel via network

-- FK index: speeds up retrieval of all endpoints for a descriptor
CREATE INDEX IF NOT EXISTS ix_aas_endpoint_descriptor_id ON aas_descriptor_endpoint(descriptor_id);

-- Enables filtering endpoints by interface type (e.g., "AAS-3.0", "SUBMODEL-3.0")
CREATE INDEX IF NOT EXISTS ix_aas_endpoint_interface ON aas_descriptor_endpoint(interface);

-- Enables exact match lookup on endpoint href (URL)
CREATE INDEX IF NOT EXISTS ix_aas_endpoint_href ON aas_descriptor_endpoint(href);

-- Trigram index: enables fuzzy/partial search on endpoint URLs
CREATE INDEX IF NOT EXISTS ix_aas_endpoint_href_trgm ON aas_descriptor_endpoint USING GIN (href gin_trgm_ops);

-- ==========================================
-- Security Attributes
-- ==========================================
-- Security attributes define access control for endpoints

-- FK index: speeds up retrieval of all security attributes for an endpoint
CREATE INDEX IF NOT EXISTS ix_secattr_endpoint_id ON security_attributes(endpoint_id);

-- Enables filtering by security type (NONE, RFC_TLSA, W3C_DID)
CREATE INDEX IF NOT EXISTS ix_secattr_type ON security_attributes(security_type);

-- ==========================================
-- Endpoint Protocol Version
-- ==========================================
-- Protocol versions supported by an endpoint

-- FK index: speeds up retrieval of all protocol versions for an endpoint
CREATE INDEX IF NOT EXISTS ix_epv_endpoint_id ON endpoint_protocol_version(endpoint_id);

-- ==========================================
-- AAS Descriptor
-- ==========================================
-- AAS descriptors are registry entries pointing to AAS instances

-- FK index: speeds up JOIN to load administrative information
CREATE INDEX IF NOT EXISTS ix_aasd_admininfo_id ON aas_descriptor(administrative_information_id);

-- Enables filtering descriptors by id_short (API query parameter)
CREATE INDEX IF NOT EXISTS ix_aasd_id_short ON aas_descriptor(id_short);

-- Enables lookup by global_asset_id (primary discovery mechanism)
CREATE INDEX IF NOT EXISTS ix_aasd_global_asset_id ON aas_descriptor(global_asset_id);

-- Trigram index: enables fuzzy/partial search on AAS identifier
CREATE INDEX IF NOT EXISTS ix_aasd_id_trgm ON aas_descriptor USING GIN (id gin_trgm_ops);

-- Trigram index: enables fuzzy/partial search on global asset ID
CREATE INDEX IF NOT EXISTS ix_aasd_global_asset_id_trgm ON aas_descriptor USING GIN (global_asset_id gin_trgm_ops);

-- Enables filtering by asset kind (Instance, Type, NotApplicable)
CREATE INDEX IF NOT EXISTS ix_aasd_asset_kind ON aas_descriptor(asset_kind);

-- ==========================================
-- Submodel Descriptor
-- ==========================================
-- Submodel descriptors are registry entries pointing to submodel instances

-- FK index: speeds up retrieval of all submodel descriptors for an AAS descriptor
CREATE INDEX IF NOT EXISTS ix_smd_aas_descriptor_id ON submodel_descriptor(aas_descriptor_id);

-- FK index: speeds up JOIN to resolve semantic_id for submodel descriptor
CREATE INDEX IF NOT EXISTS ix_smd_semantic_id ON submodel_descriptor(semantic_id);

-- Enables filtering submodel descriptors by id_short
CREATE INDEX IF NOT EXISTS ix_smd_id_short ON submodel_descriptor(id_short);

-- Trigram index: enables fuzzy/partial search on submodel identifier
CREATE INDEX IF NOT EXISTS ix_smd_id_trgm ON submodel_descriptor USING GIN (id gin_trgm_ops);

-- FK index: speeds up retrieval of supplemental semantic IDs for submodel descriptor
CREATE INDEX IF NOT EXISTS ix_smdss_descriptor_id ON submodel_descriptor_supplemental_semantic_id(descriptor_id);

-- FK index: speeds up JOIN to resolve reference for supplemental semantic ID
CREATE INDEX IF NOT EXISTS ix_smdss_reference_id ON submodel_descriptor_supplemental_semantic_id(reference_id);

-- ==========================================
-- Registry Descriptor
-- ==========================================
-- Registry descriptors for registry-of-registries functionality

-- Enables filtering registry descriptors by id_short
CREATE INDEX IF NOT EXISTS ix_regd_id_short ON registry_descriptor(id_short);

-- Enables lookup by global_asset_id (discovery mechanism)
CREATE INDEX IF NOT EXISTS ix_regd_global_asset_id ON registry_descriptor(global_asset_id);

-- Enables filtering by registry type (AAS_REGISTRY, SUBMODEL_REGISTRY)
CREATE INDEX IF NOT EXISTS ix_regd_registry_type ON registry_descriptor(registry_type);

-- Trigram index: enables fuzzy/partial search on registry identifier
CREATE INDEX IF NOT EXISTS ix_regd_id_trgm ON registry_descriptor USING GIN (id gin_trgm_ops);

-- ==========================================
-- Trigger functions for cascading deletion
-- ==========================================
-- Trigger function to clean up orphaned records when a registry_descriptor is deleted
CREATE OR REPLACE FUNCTION cleanup_registry_descriptor()
RETURNS TRIGGER AS $$
DECLARE
    v_creator BIGINT;
BEGIN
    -- Delete the administrative_information record if it exists
    IF OLD.administrative_information_id IS NOT NULL THEN
        -- Get the creator reference ID before deleting the administrative_information
        SELECT creator INTO v_creator FROM administrative_information WHERE id = OLD.administrative_information_id;
        
        -- Delete the administrative_information record
        DELETE FROM administrative_information WHERE id = OLD.administrative_information_id;
        
        -- Delete the creator reference if it exists and is orphaned
        IF v_creator IS NOT NULL THEN
            DELETE FROM reference WHERE id = v_creator;
        END IF;
    END IF;
    
    -- Delete the description_id lang_string_text_type_reference if it exists
    IF OLD.description_id IS NOT NULL THEN
        DELETE FROM lang_string_text_type_reference WHERE id = OLD.description_id;
    END IF;
    
    -- Delete the displayname_id lang_string_name_type_reference if it exists
    IF OLD.displayname_id IS NOT NULL THEN
        DELETE FROM lang_string_name_type_reference WHERE id = OLD.displayname_id;
    END IF;
    
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to execute cleanup function after registry_descriptor deletion
DROP TRIGGER IF EXISTS trigger_cleanup_registry_descriptor ON registry_descriptor;
CREATE TRIGGER trigger_cleanup_registry_descriptor
    AFTER DELETE ON registry_descriptor
    FOR EACH ROW
    EXECUTE FUNCTION cleanup_registry_descriptor();