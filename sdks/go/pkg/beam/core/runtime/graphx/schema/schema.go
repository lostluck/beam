// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package schema contains utility functions to turn Go Types to Beam Schemas and vice versa.
package schema

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/apache/beam/sdks/go/pkg/beam/core/util/reflectx"
	"github.com/apache/beam/sdks/go/pkg/beam/internal/errors"
	pipepb "github.com/apache/beam/sdks/go/pkg/beam/model/pipeline_v1"
)

// FromType returns a Beam Schema of the passed in type.
// Returns an error if the type cannot be converted to a Schema.
func FromType(ot reflect.Type) (*pipepb.Schema, error) {
	t := ot // keep the original type for errors.
	// The top level schema for a pointer to struct and the struct is the same.
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, errors.Errorf("cannot convert %v to schema. FromType only converts structs to schemas", ot)
	}
	return nil, nil
}

// ToType returns a Go type of the passed in Schema.
// Types returned by ToType are always of Struct kind.
// Returns an error if the Schema cannot be converted to a type.
func ToType(s *pipepb.Schema) (reflect.Type, error) {
	fields := make([]reflect.StructField, 0, len(s.GetFields()))
	for _, sf := range s.GetFields() {
		rf := fieldToStructField(sf)
		fields = append(fields, rf)
	}
	return reflect.StructOf(fields), nil
}

func fieldToStructField(sf *pipepb.Field) reflect.StructField {
	name := sf.GetName()

	return reflect.StructField{
		Name: strings.ToUpper(name[:1]) + name[1:], // Go field name must be capitalized for export and encoding.
		Type: fieldTypeToReflectType(sf.GetType()),
		Tag:  reflect.StructTag(fmt.Sprintf("beam:\"%s\"", name)),
	}
}

var atomicTypeToReflectType = map[pipepb.AtomicType]reflect.Type{
	pipepb.AtomicType_BYTE:    reflectx.Uint8,
	pipepb.AtomicType_INT16:   reflectx.Int16,
	pipepb.AtomicType_INT32:   reflectx.Int32,
	pipepb.AtomicType_INT64:   reflectx.Int64,
	pipepb.AtomicType_FLOAT:   reflectx.Float32,
	pipepb.AtomicType_DOUBLE:  reflectx.Float64,
	pipepb.AtomicType_STRING:  reflectx.String,
	pipepb.AtomicType_BOOLEAN: reflectx.Bool,
	pipepb.AtomicType_BYTES:   reflectx.ByteSlice,
}

func fieldTypeToReflectType(sft *pipepb.FieldType) reflect.Type {
	var t reflect.Type
	switch sft.GetTypeInfo().(type) {
	case *pipepb.FieldType_AtomicType:
		var ok bool
		if t, ok = atomicTypeToReflectType[sft.GetAtomicType()]; !ok {
			panic(fmt.Sprintf("unknown atomic type: %v", sft.GetAtomicType()))
		}
	case *pipepb.FieldType_ArrayType:
		t = reflect.SliceOf(fieldTypeToReflectType(sft.GetArrayType().GetElementType()))
	case *pipepb.FieldType_IterableType:
		// TODO figure out the correct handling for iterables.
		t = reflect.SliceOf(fieldTypeToReflectType(sft.GetIterableType().GetElementType()))
	case *pipepb.FieldType_MapType:
		kt := fieldTypeToReflectType(sft.GetMapType().GetKeyType())
		vt := fieldTypeToReflectType(sft.GetMapType().GetValueType())
		t = reflect.MapOf(kt, vt) // Panics for invalid map keys (slices/iterables)
	case *pipepb.FieldType_RowType:
		rt, err := ToType(sft.GetRowType().GetSchema())
		if err != nil {
			panic(err)
		}
		t = rt
	case *pipepb.FieldType_LogicalType:
		// Logical Types are for things that have more specialized user representation already, or
		// things like Time or protocol buffers.
		// They would be encoded with the schema encoding.
		// TODOlookup logical types.
	default:
		panic(fmt.Sprintf("unknown fieldtype: %T", sft.GetTypeInfo()))
	}
	if sft.GetNullable() {
		return reflect.PtrTo(t)
	}
	return t
}
