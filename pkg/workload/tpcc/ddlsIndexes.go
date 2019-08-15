// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tpcc

import (
	"fmt"
	"strings"
)

type indexType int

const (
	_ indexType = iota
	coveringIndex
	uniqueIndex
)

type indexDDL struct {
	name        string
	table       string
	columns     []string
	typeOfIndex indexType
}

func (i indexDDL) createIndexStatement() string {
	stmt := `CREATE`

	if i.typeOfIndex == uniqueIndex {
		stmt += ` UNIQUE`
	}

	stmt += fmt.Sprintf(" INDEX IF NOT EXISTS %s ON %s (%s);\n", i.name, i.table, strings.Join(i.columns, ","))

	return stmt
}

func (i indexDDL) inlineIndex() string {
	var stmt string
	if i.typeOfIndex == uniqueIndex {
		stmt += `UNIQUE `
	}
	return fmt.Sprintf("INDEX %s (%s)", i.name, strings.Join(i.columns, ","))
}
