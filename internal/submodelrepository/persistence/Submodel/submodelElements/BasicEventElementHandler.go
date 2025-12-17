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
// Author: Jannik Fried ( Fraunhofer IESE )

// Package submodelelements provides handlers for different types of submodel elements in the BaSyx framework.
// This package contains PostgreSQL-based persistence implementations for various submodel element types
// including basic event elements.
package submodelelements

import (
	"database/sql"

	"github.com/eclipse-basyx/basyx-go-components/internal/common"
	gen "github.com/eclipse-basyx/basyx-go-components/internal/common/model"
	_ "github.com/lib/pq" // PostgreSQL Treiber
)

// PostgreSQLBasicEventElementHandler provides PostgreSQL-based persistence operations
// for BasicEventElement submodel elements. It implements CRUD operations and handles
// the event-specific properties such as observed references, message brokers, and timing intervals.
type PostgreSQLBasicEventElementHandler struct {
	db        *sql.DB
	decorated *PostgreSQLSMECrudHandler
}

// NewPostgreSQLBasicEventElementHandler creates a new handler for BasicEventElement persistence.
// It initializes the handler with a database connection and sets up the decorated CRUD handler
// for common submodel element operations.
//
// Parameters:
//   - db: PostgreSQL database connection
//
// Returns:
//   - *PostgreSQLBasicEventElementHandler: Configured handler instance
//   - error: Error if handler initialization fails
func NewPostgreSQLBasicEventElementHandler(db *sql.DB) (*PostgreSQLBasicEventElementHandler, error) {
	decoratedHandler, err := NewPostgreSQLSMECrudHandler(db)
	if err != nil {
		return nil, err
	}
	return &PostgreSQLBasicEventElementHandler{db: db, decorated: decoratedHandler}, nil
}

// Create inserts a new BasicEventElement into the database as a top-level submodel element.
// This method handles both the common submodel element properties and the specific event
// properties such as observed references, message brokers, and timing intervals.
//
// Parameters:
//   - tx: Active database transaction
//   - submodelID: ID of the parent submodel
//   - submodelElement: The BasicEventElement to create
//
// Returns:
//   - int: Database ID of the created element
//   - error: Error if creation fails or element is not of correct type
func (p PostgreSQLBasicEventElementHandler) Create(tx *sql.Tx, submodelID string, submodelElement gen.SubmodelElement) (int, error) {
	basicEvent, ok := submodelElement.(*gen.BasicEventElement)
	if !ok {
		return 0, common.NewErrBadRequest("submodelElement is not of type BasicEventElement")
	}

	// First, perform base SubmodelElement operations within the transaction
	id, err := p.decorated.Create(tx, submodelID, submodelElement)
	if err != nil {
		return 0, err
	}

	// BasicEventElement-specific database insertion
	err = insertBasicEventElement(basicEvent, tx, id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// CreateNested inserts a new BasicEventElement as a nested element within a collection or list.
// This method creates the element at a specific hierarchical path and position within its parent container.
// It handles both the parent-child relationship and the specific basic event element data.
//
// Parameters:
//   - tx: Active database transaction
//   - submodelID: ID of the parent submodel
//   - parentID: Database ID of the parent element
//   - idShortPath: Hierarchical path where the element should be created
//   - submodelElement: The BasicEventElement to create
//   - pos: Position within the parent container
//
// Returns:
//   - int: Database ID of the created nested element
//   - error: Error if creation fails or element is not of correct type
func (p PostgreSQLBasicEventElementHandler) CreateNested(tx *sql.Tx, submodelID string, parentID int, idShortPath string, submodelElement gen.SubmodelElement, pos int, rootSubmodelElementID int) (int, error) {
	basicEvent, ok := submodelElement.(*gen.BasicEventElement)
	if !ok {
		return 0, common.NewErrBadRequest("submodelElement is not of type BasicEventElement")
	}

	// Create the nested basic event element with the provided idShortPath using the decorated handler
	id, err := p.decorated.CreateWithPath(tx, submodelID, parentID, idShortPath, submodelElement, pos, rootSubmodelElementID)
	if err != nil {
		return 0, err
	}

	// BasicEventElement-specific database insertion for nested element
	err = insertBasicEventElement(basicEvent, tx, id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// Update modifies an existing BasicEventElement identified by its idShort or path.
// It updates both the base submodel element properties and BasicEventElement-specific data.
//
// Parameters:
//   - idShortOrPath: idShort or hierarchical path to the element to update
//   - submodelElement: Updated element data
//
// Returns:
//   - error: Error if update fails
func (p PostgreSQLBasicEventElementHandler) Update(idShortOrPath string, submodelElement gen.SubmodelElement) error {
	basicEvent, ok := submodelElement.(*gen.BasicEventElement)
	if !ok {
		return common.NewErrBadRequest("submodelElement is not of type BasicEventElement")
	}

	// Update base submodel element first (which starts its own transaction)
	err := p.decorated.Update(idShortOrPath, submodelElement)
	if err != nil {
		return err
	}

	// Start a new transaction for BasicEventElement-specific updates
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	// Get the element ID
	var elementID int
	err = tx.QueryRow(`SELECT id FROM submodel_element WHERE idshort_path = $1`, idShortOrPath).Scan(&elementID)
	if err != nil {
		if err == sql.ErrNoRows {
			return common.NewErrNotFound("basic event element not found")
		}
		return err
	}

	// Delete old references if they exist
	var oldObservedRefID, oldMessageBrokerRefID sql.NullInt64
	err = tx.QueryRow(`SELECT observed_ref, message_broker_ref FROM basic_event_element WHERE id = $1`, elementID).
		Scan(&oldObservedRefID, &oldMessageBrokerRefID)
	if err != nil {
		return err
	}

	// Delete old reference keys and references
	if oldObservedRefID.Valid {
		_, err = tx.Exec(`DELETE FROM reference_key WHERE reference_id = $1`, oldObservedRefID.Int64)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM reference WHERE id = $1`, oldObservedRefID.Int64)
		if err != nil {
			return err
		}
	}

	if oldMessageBrokerRefID.Valid {
		_, err = tx.Exec(`DELETE FROM reference_key WHERE reference_id = $1`, oldMessageBrokerRefID.Int64)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM reference WHERE id = $1`, oldMessageBrokerRefID.Int64)
		if err != nil {
			return err
		}
	}

	// Insert new references
	var observedRefID sql.NullInt64
	if !isEmptyReference(basicEvent.Observed) {
		var refID int
		err := tx.QueryRow(`INSERT INTO reference (type) VALUES ($1) RETURNING id`, basicEvent.Observed.Type).Scan(&refID)
		if err != nil {
			return err
		}
		observedRefID = sql.NullInt64{Int64: int64(refID), Valid: true}

		keys := basicEvent.Observed.Keys
		for i := range keys {
			_, err = tx.Exec(`INSERT INTO reference_key (reference_id, position, type, value) VALUES ($1, $2, $3, $4)`,
				refID, i, keys[i].Type, keys[i].Value)
			if err != nil {
				return err
			}
		}
	}

	var messageBrokerRefID sql.NullInt64
	if !isEmptyReference(basicEvent.MessageBroker) {
		var refID int
		err := tx.QueryRow(`INSERT INTO reference (type) VALUES ($1) RETURNING id`, basicEvent.MessageBroker.Type).Scan(&refID)
		if err != nil {
			return err
		}
		messageBrokerRefID = sql.NullInt64{Int64: int64(refID), Valid: true}

		keys := basicEvent.MessageBroker.Keys
		for i := range keys {
			_, err = tx.Exec(`INSERT INTO reference_key (reference_id, position, type, value) VALUES ($1, $2, $3, $4)`,
				refID, i, keys[i].Type, keys[i].Value)
			if err != nil {
				return err
			}
		}
	}

	// Handle nullable fields
	var lastUpdate sql.NullString
	if basicEvent.LastUpdate != "" {
		lastUpdate = sql.NullString{String: basicEvent.LastUpdate, Valid: true}
	}

	var minInterval sql.NullString
	if basicEvent.MinInterval != "" {
		minInterval = sql.NullString{String: basicEvent.MinInterval, Valid: true}
	}

	var maxInterval sql.NullString
	if basicEvent.MaxInterval != "" {
		maxInterval = sql.NullString{String: basicEvent.MaxInterval, Valid: true}
	}

	var messageTopic sql.NullString
	if basicEvent.MessageTopic != "" {
		messageTopic = sql.NullString{String: basicEvent.MessageTopic, Valid: true}
	}

	// Update BasicEventElement-specific data
	_, err = tx.Exec(`UPDATE basic_event_element 
					  SET observed_ref = $1, direction = $2, state = $3, message_topic = $4, 
					      message_broker_ref = $5, last_update = $6, min_interval = $7, max_interval = $8
					  WHERE id = $9`,
		observedRefID, basicEvent.Direction, basicEvent.State, messageTopic,
		messageBrokerRefID, lastUpdate, minInterval, maxInterval, elementID)
	if err != nil {
		return err
	}

	return nil
}

// Delete removes a BasicEventElement identified by its idShort or path from the database.
// This method delegates the deletion operation to the decorated CRUD handler which handles
// the cascading deletion of all related data and child elements.
//
// Parameters:
//   - idShortOrPath: idShort or hierarchical path to the element to delete
//
// Returns:
//   - error: Error if deletion fails
func (p PostgreSQLBasicEventElementHandler) Delete(idShortOrPath string) error {
	return p.decorated.Delete(idShortOrPath)
}

func insertBasicEventElement(basicEvent *gen.BasicEventElement, tx *sql.Tx, id int) error {
	var observedRefID sql.NullInt64
	if !isEmptyReference(basicEvent.Observed) {
		var refID int
		err := tx.QueryRow(`INSERT INTO reference (type) VALUES ($1) RETURNING id`, basicEvent.Observed.Type).Scan(&refID)
		if err != nil {
			return err
		}
		observedRefID = sql.NullInt64{Int64: int64(refID), Valid: true}

		keys := basicEvent.Observed.Keys
		for i := range keys {
			_, err = tx.Exec(`INSERT INTO reference_key (reference_id, position, type, value) VALUES ($1, $2, $3, $4)`,
				refID, i, keys[i].Type, keys[i].Value)
			if err != nil {
				return err
			}
		}
	}

	var messageBrokerRefID sql.NullInt64
	if !isEmptyReference(basicEvent.MessageBroker) {
		var refID int
		err := tx.QueryRow(`INSERT INTO reference (type) VALUES ($1) RETURNING id`, basicEvent.MessageBroker.Type).Scan(&refID)
		if err != nil {
			return err
		}
		messageBrokerRefID = sql.NullInt64{Int64: int64(refID), Valid: true}

		keys := basicEvent.MessageBroker.Keys
		for i := range keys {
			_, err = tx.Exec(`INSERT INTO reference_key (reference_id, position, type, value) VALUES ($1, $2, $3, $4)`,
				refID, i, keys[i].Type, keys[i].Value)
			if err != nil {
				return err
			}
		}
	}

	// Handle nullable fields
	var lastUpdate sql.NullString
	if basicEvent.LastUpdate != "" {
		lastUpdate = sql.NullString{String: basicEvent.LastUpdate, Valid: true}
	}

	var minInterval sql.NullString
	if basicEvent.MinInterval != "" {
		minInterval = sql.NullString{String: basicEvent.MinInterval, Valid: true}
	}

	var maxInterval sql.NullString
	if basicEvent.MaxInterval != "" {
		maxInterval = sql.NullString{String: basicEvent.MaxInterval, Valid: true}
	}

	var messageTopic sql.NullString
	if basicEvent.MessageTopic != "" {
		messageTopic = sql.NullString{String: basicEvent.MessageTopic, Valid: true}
	}

	_, err := tx.Exec(`INSERT INTO basic_event_element (id, observed_ref, direction, state, message_topic, message_broker_ref, last_update, min_interval, max_interval) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, observedRefID, basicEvent.Direction, basicEvent.State, messageTopic, messageBrokerRefID, lastUpdate, minInterval, maxInterval)
	return err
}
