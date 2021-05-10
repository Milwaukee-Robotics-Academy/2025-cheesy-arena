// Copyright 2021 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Defines a "table" wrapper struct and helper methods for persisting data using Bolt.

package model

import (
	"encoding/json"
	"fmt"
	"go.etcd.io/bbolt"
	"reflect"
	"strconv"
)

// Encapsulates all persistence operations for a particular data type represented by a struct.
type table struct {
	bolt         *bbolt.DB
	recordType   reflect.Type
	name         string
	bucketKey    []byte
	idFieldIndex *int
}

// Registers a new table for a struct, given its zero value.
func (database *Database) newTable(recordType interface{}) (*table, error) {
	recordTypeValue := reflect.ValueOf(recordType)
	if recordTypeValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("record type must be a struct; got %v", recordTypeValue.Kind())
	}

	var table table
	table.bolt = database.bolt
	table.recordType = reflect.TypeOf(recordType)
	table.name = table.recordType.Name()
	table.bucketKey = []byte(table.name)

	// Determine which field in the struct is tagged as the ID and cache its index.
	idFound := false
	for i := 0; i < recordTypeValue.Type().NumField(); i++ {
		field := recordTypeValue.Type().Field(i)
		tag := field.Tag.Get("db")
		if tag == "id" {
			if field.Type.Kind() != reflect.Int64 {
				return nil,
					fmt.Errorf(
						"field in struct %s tagged with 'id' must be an int64; got %v", table.name, field.Type.Kind(),
					)
			}
			table.idFieldIndex = new(int)
			*table.idFieldIndex = i
			idFound = true
			break
		}
	}
	if !idFound {
		return nil, fmt.Errorf("struct %s has no field tagged as the id", table.name)
	}

	// Create the Bolt bucket corresponding to the struct.
	err := table.bolt.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(table.bucketKey)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &table, nil
}

// Populates the given double pointer to a record with the data from the record with the given ID, or nil if it doesn't
// exist.
func (table *table) getById(id int64, record interface{}) error {
	if err := table.validateType(record, reflect.Ptr, reflect.Ptr, reflect.Struct); err != nil {
		return err
	}

	return table.bolt.View(func(tx *bbolt.Tx) error {
		bucket, err := table.getBucket(tx)
		if err != nil {
			return err
		}

		if recordJson := bucket.Get(idToKey(id)); recordJson != nil {
			return json.Unmarshal(recordJson, record)
		}

		// If the record does not exist, set the record pointer to nil.
		recordPointerValue := reflect.ValueOf(record).Elem()
		recordPointerValue.Set(reflect.Zero(recordPointerValue.Type()))

		return nil
	})
}

// Populates the given slice passed by pointer with the data from every record in the table, ordered by ID.
func (table *table) getAll(recordSlice interface{}) error {
	if err := table.validateType(recordSlice, reflect.Ptr, reflect.Slice, reflect.Struct); err != nil {
		return err
	}

	return table.bolt.View(func(tx *bbolt.Tx) error {
		bucket, err := table.getBucket(tx)
		if err != nil {
			return err
		}

		recordSliceValue := reflect.ValueOf(recordSlice).Elem()
		recordSliceValue.Set(reflect.MakeSlice(recordSliceValue.Type(), 0, 0))
		return bucket.ForEach(func(key, value []byte) error {
			record := reflect.New(table.recordType)
			err := json.Unmarshal(value, record.Interface())
			if err != nil {
				return err
			}
			recordSliceValue.Set(reflect.Append(recordSliceValue, record.Elem()))
			return nil
		})
	})
}

// Persists the given record as a new row in the table.
func (table *table) create(record interface{}) error {
	if err := table.validateType(record, reflect.Ptr, reflect.Struct); err != nil {
		return err
	}

	// Validate that the record has its ID set to zero since it will be given an auto-generated one.
	value := reflect.ValueOf(record).Elem()
	id := value.Field(*table.idFieldIndex).Int()
	if id != 0 {
		return fmt.Errorf("can't create %s with non-zero ID: %d", table.name, id)
	}

	return table.bolt.Update(func(tx *bbolt.Tx) error {
		bucket, err := table.getBucket(tx)
		if err != nil {
			return err
		}

		// Generate a new ID for the record.
		newSequence, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		id = int64(newSequence)
		value.Field(*table.idFieldIndex).SetInt(id)

		// Ensure that a record having the same ID does not already exist in the table.
		key := idToKey(id)
		oldRecord := bucket.Get(key)
		if oldRecord != nil {
			return fmt.Errorf("%s with ID %d already exists: %s", table.name, id, string(oldRecord))
		}

		recordJson, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return bucket.Put(key, recordJson)
	})
}

// Persists the given record as an update to the existing row in the table. Returns an error if the record does not
// already exist.
func (table *table) update(record interface{}) error {
	if err := table.validateType(record, reflect.Ptr, reflect.Struct); err != nil {
		return err
	}

	// Validate that the record has a non-zero ID.
	value := reflect.ValueOf(record).Elem()
	id := value.Field(*table.idFieldIndex).Int()
	if id == 0 {
		return fmt.Errorf("can't update %s with zero ID", table.name)
	}

	return table.bolt.Update(func(tx *bbolt.Tx) error {
		bucket, err := table.getBucket(tx)
		if err != nil {
			return err
		}

		// Ensure that a record having the same ID exists in the table.
		key := idToKey(id)
		oldRecord := bucket.Get(key)
		if oldRecord == nil {
			return fmt.Errorf("can't update non-existent %s with ID %d", table.name, id)
		}

		recordJson, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return bucket.Put(key, recordJson)
	})
}

// Deletes the record having the given ID from the table. Returns an error if the record does not exist.
func (table *table) delete(id int64) error {
	return table.bolt.Update(func(tx *bbolt.Tx) error {
		bucket, err := table.getBucket(tx)
		if err != nil {
			return err
		}

		// Ensure that a record having the same ID exists in the table.
		key := idToKey(id)
		oldRecord := bucket.Get(key)
		if oldRecord == nil {
			return fmt.Errorf("can't delete non-existent %s with ID %d", table.name, id)
		}

		return bucket.Delete(key)
	})
}

// Deletes all records from the table.
func (table *table) truncate() error {
	return table.bolt.Update(func(tx *bbolt.Tx) error {
		_, err := table.getBucket(tx)
		if err != nil {
			return err
		}

		// Carry out the truncation by way of deleting the whole bucket and then recreate it.
		err = tx.DeleteBucket(table.bucketKey)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucket(table.bucketKey)
		return err
	})
}

// Obtains the Bolt bucket belonging to the table.
func (table *table) getBucket(tx *bbolt.Tx) (*bbolt.Bucket, error) {
	bucket := tx.Bucket(table.bucketKey)
	if bucket == nil {
		return nil, fmt.Errorf("unknown table %s", table.name)
	}
	return bucket, nil
}

// Validates that the given record is of the expected derived type (e.g. pointer, slice, etc.), that the base type is
// the same as that stored in the table, and that the table is configured correctly.
func (table *table) validateType(record interface{}, kinds ...reflect.Kind) error {
	// Check the hierarchy of kinds against the expected list until reaching the base record type.
	recordType := reflect.ValueOf(record).Type()
	expectedKind := ""
	actualKind := ""
	for i, kind := range kinds {
		if i > 0 {
			expectedKind += " -> "
			actualKind += " -> "
		}
		expectedKind += kind.String()
		actualKind += recordType.Kind().String()
		if recordType.Kind() != kind {
			return fmt.Errorf("input must be a %s; got a %s", expectedKind, actualKind)
		}
		if i < len(kinds)-1 {
			recordType = recordType.Elem()
		}
	}

	if recordType != table.recordType {
		return fmt.Errorf("given record of type %s does not match expected type for table %s", recordType, table.name)
	}

	if table.idFieldIndex == nil {
		return fmt.Errorf("struct %s has no field tagged as the id", table.name)
	}

	return nil
}

// Serializes the given integer ID to a byte array containing its Base-10 string representation.
func idToKey(id int64) []byte {
	return []byte(strconv.FormatInt(id, 10))
}
