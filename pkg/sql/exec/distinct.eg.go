// Code generated by execgen; DO NOT EDIT.
// Copyright 2018 The Cockroach Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package exec

import (
	"bytes"

	"github.com/cockroachdb/apd"
	"github.com/cockroachdb/cockroach/pkg/sql/exec/types"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/pkg/errors"
)

// orderedDistinctColsToOperators is a utility function that given an input and
// a slice of columns, creates a chain of distinct operators and returns the
// last distinct operator in that chain as well as its output column.
func orderedDistinctColsToOperators(
	input Operator, distinctCols []uint32, typs []types.T,
) (Operator, []bool, error) {
	distinctCol := make([]bool, ColBatchSize)
	var err error
	for i := range distinctCols {
		input, err = newSingleOrderedDistinct(input, int(distinctCols[i]), distinctCol, typs[i])
		if err != nil {
			return nil, nil, err
		}
	}
	return input, distinctCol, nil
}

// NewOrderedDistinct creates a new ordered distinct operator on the given
// input columns with the given types.
func NewOrderedDistinct(input Operator, distinctCols []uint32, typs []types.T) (Operator, error) {
	op, outputCol, err := orderedDistinctColsToOperators(input, distinctCols, typs)
	if err != nil {
		return nil, err
	}
	return &boolVecToSelOp{
		input:     op,
		outputCol: outputCol,
	}, nil
}

func newSingleOrderedDistinct(
	input Operator, distinctColIdx int, outputCol []bool, t types.T,
) (Operator, error) {
	switch t {
	case types.Bool:
		return &sortedDistinctBoolOp{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Bytes:
		return &sortedDistinctBytesOp{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Decimal:
		return &sortedDistinctDecimalOp{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Int8:
		return &sortedDistinctInt8Op{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Int16:
		return &sortedDistinctInt16Op{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Int32:
		return &sortedDistinctInt32Op{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Int64:
		return &sortedDistinctInt64Op{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Float32:
		return &sortedDistinctFloat32Op{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	case types.Float64:
		return &sortedDistinctFloat64Op{
			input:             input,
			sortedDistinctCol: distinctColIdx,
			outputCol:         outputCol,
		}, nil
	default:
		return nil, errors.Errorf("unsupported distinct type %s", t)
	}
}

// partitioner is a simple implementation of sorted distinct that's useful for
// other operators that need to partition an arbitrarily-sized ColVec.
type partitioner interface {
	// partition partitions the input colVec of size n, writing true to the
	// outputCol for every value that differs from the previous one.
	partition(colVec ColVec, outputCol []bool, n uint64)
}

// newPartitioner returns a new partitioner on type t.
func newPartitioner(t types.T) (partitioner, error) {
	switch t {
	case types.Bool:
		return partitionerBool{}, nil
	case types.Bytes:
		return partitionerBytes{}, nil
	case types.Decimal:
		return partitionerDecimal{}, nil
	case types.Int8:
		return partitionerInt8{}, nil
	case types.Int16:
		return partitionerInt16{}, nil
	case types.Int32:
		return partitionerInt32{}, nil
	case types.Int64:
		return partitionerInt64{}, nil
	case types.Float32:
		return partitionerFloat32{}, nil
	case types.Float64:
		return partitionerFloat64{}, nil
	default:
		return nil, errors.Errorf("unsupported partition type %s", t)
	}
}

// sortedDistinctBoolOp runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctBoolOp struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal bool
}

var _ Operator = &sortedDistinctBoolOp{}

func (p *sortedDistinctBoolOp) Init() {
	p.input.Init()
}

func (p *sortedDistinctBoolOp) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Bool()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerBool partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerBool struct{}

func (p partitionerBool) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Bool()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = v != lastVal
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctBytesOp runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctBytesOp struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal []byte
}

var _ Operator = &sortedDistinctBytesOp{}

func (p *sortedDistinctBytesOp) Init() {
	p.input.Init()
}

func (p *sortedDistinctBytesOp) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Bytes()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = !bytes.Equal(v, lastVal)
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = !bytes.Equal(v, lastVal)
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerBytes partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerBytes struct{}

func (p partitionerBytes) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Bytes()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = !bytes.Equal(v, lastVal)
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctDecimalOp runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctDecimalOp struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal apd.Decimal
}

var _ Operator = &sortedDistinctDecimalOp{}

func (p *sortedDistinctDecimalOp) Init() {
	p.input.Init()
}

func (p *sortedDistinctDecimalOp) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Decimal()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = tree.CompareDecimals(&v, &lastVal) != 0
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = tree.CompareDecimals(&v, &lastVal) != 0
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerDecimal partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerDecimal struct{}

func (p partitionerDecimal) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Decimal()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = tree.CompareDecimals(&v, &lastVal) != 0
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctInt8Op runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctInt8Op struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal int8
}

var _ Operator = &sortedDistinctInt8Op{}

func (p *sortedDistinctInt8Op) Init() {
	p.input.Init()
}

func (p *sortedDistinctInt8Op) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Int8()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerInt8 partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerInt8 struct{}

func (p partitionerInt8) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Int8()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = v != lastVal
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctInt16Op runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctInt16Op struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal int16
}

var _ Operator = &sortedDistinctInt16Op{}

func (p *sortedDistinctInt16Op) Init() {
	p.input.Init()
}

func (p *sortedDistinctInt16Op) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Int16()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerInt16 partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerInt16 struct{}

func (p partitionerInt16) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Int16()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = v != lastVal
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctInt32Op runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctInt32Op struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal int32
}

var _ Operator = &sortedDistinctInt32Op{}

func (p *sortedDistinctInt32Op) Init() {
	p.input.Init()
}

func (p *sortedDistinctInt32Op) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Int32()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerInt32 partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerInt32 struct{}

func (p partitionerInt32) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Int32()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = v != lastVal
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctInt64Op runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctInt64Op struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal int64
}

var _ Operator = &sortedDistinctInt64Op{}

func (p *sortedDistinctInt64Op) Init() {
	p.input.Init()
}

func (p *sortedDistinctInt64Op) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Int64()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerInt64 partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerInt64 struct{}

func (p partitionerInt64) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Int64()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = v != lastVal
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctFloat32Op runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctFloat32Op struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal float32
}

var _ Operator = &sortedDistinctFloat32Op{}

func (p *sortedDistinctFloat32Op) Init() {
	p.input.Init()
}

func (p *sortedDistinctFloat32Op) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Float32()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerFloat32 partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerFloat32 struct{}

func (p partitionerFloat32) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Float32()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = v != lastVal
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}

// sortedDistinctFloat64Op runs a distinct on the column in sortedDistinctCol,
// writing true to the resultant bool column for every value that differs from
// the previous one.

type sortedDistinctFloat64Op struct {
	input Operator

	// sortedDistinctCol is the index of the column to distinct upon.
	sortedDistinctCol int

	// outputCol is the boolean output column. It is shared by all of the
	// other distinct operators in a distinct operator set.
	outputCol []bool

	// Set to true at runtime when we've seen the first row. Distinct always
	// outputs the first row that it sees.
	foundFirstRow bool

	// lastVal is the last value seen by the operator, so that the distincting
	// still works across batch boundaries.
	lastVal float64
}

var _ Operator = &sortedDistinctFloat64Op{}

func (p *sortedDistinctFloat64Op) Init() {
	p.input.Init()
}

func (p *sortedDistinctFloat64Op) Next() ColBatch {
	batch := p.input.Next()
	if batch.Length() == 0 {
		return batch
	}
	outputCol := p.outputCol
	col := batch.ColVec(p.sortedDistinctCol).Float64()

	// We always output the first row.
	lastVal := p.lastVal
	sel := batch.Selection()
	if !p.foundFirstRow {
		if sel != nil {
			lastVal = col[sel[0]]
			outputCol[sel[0]] = true
		} else {
			lastVal = col[0]
			outputCol[0] = true
		}
	}

	startIdx := uint16(0)
	if !p.foundFirstRow {
		startIdx = 1
	}

	n := batch.Length()
	if sel != nil {
		// Bounds check elimination.
		sel = sel[startIdx:n]
		for _, i := range sel {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	} else {
		// Bounds check elimination.
		col = col[startIdx:n]
		outputCol = outputCol[startIdx:n]
		for i := range col {

			v := col[i]
			// Note that not inlining this unique var actually makes a non-trivial
			// performance difference.
			var unique bool
			unique = v != lastVal
			outputCol[i] = outputCol[i] || unique
			lastVal = v
		}
	}

	p.lastVal = lastVal
	p.foundFirstRow = true

	return batch
}

// partitionerFloat64 partitions an arbitrary-length colVec by running a distinct
// operation over it. It writes the same format to outputCol that sorted
// distinct does: true for every row that differs from the previous row in the
// input column.
type partitionerFloat64 struct{}

func (p partitionerFloat64) partition(colVec ColVec, outputCol []bool, n uint64) {
	col := colVec.Float64()
	lastVal := col[0]
	outputCol[0] = true
	outputCol = outputCol[1:n]
	col = col[1:n]
	for i := range col {
		v := col[i]
		var unique bool
		unique = v != lastVal
		outputCol[i] = outputCol[i] || unique
		lastVal = v
	}
}
