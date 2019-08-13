// Copyright 2018 The Cockroach Authors.
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
	gosql "database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
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

const (
	// WAREHOUSE table.
	tpccWarehouseSchema = `(
		w_id        integer   not null primary key,
		w_name      varchar(10),
		w_street_1  varchar(20),
		w_street_2  varchar(20),
		w_city      varchar(20),
		w_state     char(2),
		w_zip       char(9),
		w_tax       decimal(4,4),
		w_ytd       decimal(12,2)
	)`

	// DISTRICT table.
	tpccDistrictSchemaBase = `(
		d_id         integer       not null,
		d_w_id       integer       not null,
		d_name       varchar(10),
		d_street_1   varchar(20),
		d_street_2   varchar(20),
		d_city       varchar(20),
		d_state      char(2),
		d_zip        char(9),
		d_tax        decimal(4,4),
		d_ytd        decimal(12,2),
		d_next_o_id  integer,
		primary key (d_w_id, d_id)
	)`
	tpccDistrictSchemaInterleaveSuffix = `
		interleave in parent warehouse (d_w_id)`

	// CUSTOMER table.
	tpccCustomerSchemaBase = `(
		c_id           integer        not null,
		c_d_id         integer        not null,
		c_w_id         integer        not null,
		c_first        varchar(16),
		c_middle       char(2),
		c_last         varchar(16),
		c_street_1     varchar(20),
		c_street_2     varchar(20),
		c_city         varchar(20),
		c_state        char(2),
		c_zip          char(9),
		c_phone        char(16),
		c_since        timestamp,
		c_credit       char(2),
		c_credit_lim   decimal(12,2),
		c_discount     decimal(4,4),
		c_balance      decimal(12,2),
		c_ytd_payment  decimal(12,2),
		c_payment_cnt  integer,
		c_delivery_cnt integer,
		c_data         varchar(500),
		primary key (c_w_id, c_d_id, c_id)`
	tpccCustomerSchemaInterleaveSuffix = `interleave in parent district (c_w_id, c_d_id)`

	// HISTORY table.
	tpccHistorySchemaBase = `(
		rowid    uuid    not null default gen_random_uuid(),
		h_c_id   integer not null,
		h_c_d_id integer not null,
		h_c_w_id integer not null,
		h_d_id   integer not null,
		h_w_id   integer not null,
		h_date   timestamp,
		h_amount decimal(6,2),
		h_data   varchar(24),
		primary key (h_w_id, rowid)`

	// ORDER table.
	tpccOrderSchemaBase = `(
		o_id         integer      not null,
		o_d_id       integer      not null,
		o_w_id       integer      not null,
		o_c_id       integer,
		o_entry_d    timestamp,
		o_carrier_id integer,
		o_ol_cnt     integer,
		o_all_local  integer,
		primary key  (o_w_id, o_d_id, o_id DESC)`

	tpccOrderSchemaBasePG = `(
		o_id         integer      not null,
		o_d_id       integer      not null,
		o_w_id       integer      not null,
		o_c_id       integer,
		o_entry_d    timestamp,
		o_carrier_id integer,
		o_ol_cnt     integer,
		o_all_local  integer,
		primary key  (o_w_id, o_d_id, o_id)`

	tpccOrderSchemaInterleaveSuffix = `
		interleave in parent district (o_w_id, o_d_id)`

	// NEW-ORDER table.
	tpccNewOrderSchema = `(
		no_o_id  integer   not null,
		no_d_id  integer   not null,
		no_w_id  integer   not null,
		primary key (no_w_id, no_d_id, no_o_id)
	)`
	// This natural-seeming interleave makes performance worse, because this
	// table has a ton of churn and produces a lot of MVCC tombstones, which
	// then will gum up the works of scans over the parent table.
	// tpccNewOrderSchemaInterleaveSuffix = `
	// 	interleave in parent "order" (no_w_id, no_d_id, no_o_id)`

	// ITEM table.
	tpccItemSchema = `(
		i_id     integer      not null,
		i_im_id  integer,
		i_name   varchar(24),
		i_price  decimal(5,2),
		i_data   varchar(50),
		primary key (i_id)
	)`

	// STOCK table.
	tpccStockSchemaBase = `(
		s_i_id       integer       not null,
		s_w_id       integer       not null,
		s_quantity   integer,
		s_dist_01    char(24),
		s_dist_02    char(24),
		s_dist_03    char(24),
		s_dist_04    char(24),
		s_dist_05    char(24),
		s_dist_06    char(24),
		s_dist_07    char(24),
		s_dist_08    char(24),
		s_dist_09    char(24),
		s_dist_10    char(24),
		s_ytd        integer,
		s_order_cnt  integer,
		s_remote_cnt integer,
		s_data       varchar(50),
		primary key (s_w_id, s_i_id)`

	tpccStockSchemaInterleaveSuffix = `
		interleave in parent warehouse (s_w_id)`

	// ORDER-LINE table.
	tpccOrderLineSchemaBase = `(
		ol_o_id         integer   not null,
		ol_d_id         integer   not null,
		ol_w_id         integer   not null,
		ol_number       integer   not null,
		ol_i_id         integer   not null,
		ol_supply_w_id  integer,
		ol_delivery_d   timestamp,
		ol_quantity     integer,
		ol_amount       decimal(6,2),
		ol_dist_info    char(24),
		primary key (ol_w_id, ol_d_id, ol_o_id DESC, ol_number)`

	tpccOrderLineSchemaBasePG = `(
		ol_o_id         integer   not null,
		ol_d_id         integer   not null,
		ol_w_id         integer   not null,
		ol_number       integer   not null,
		ol_i_id         integer   not null,
		ol_supply_w_id  integer,
		ol_delivery_d   timestamp,
		ol_quantity     integer,
		ol_amount       decimal(6,2),
		ol_dist_info    char(24),
		primary key (ol_w_id, ol_d_id, ol_o_id, ol_number)`

	tpccOrderLineSchemaInterleaveSuffix = `
		interleave in parent "order" (ol_w_id, ol_d_id, ol_o_id)`
)

var tpccCustomerIndex = indexDDL{
	name:        "customer_idx",
	table:       "customer",
	columns:     []string{"c_w_id", "c_d_id", "c_last", "c_first"},
	typeOfIndex: coveringIndex,
}

var tpccHistoryCustomerFKIndex = indexDDL{
	name:        "history_customer_fk_idx",
	table:       "history",
	columns:     []string{"h_w_id", "h_d_id"},
	typeOfIndex: coveringIndex,
}

var tpccHistoryDistrictFKIndex = indexDDL{
	name:        "history_district_fk_idx",
	table:       "history",
	columns:     []string{"h_c_w_id", "h_c_d_id", "h_c_id"},
	typeOfIndex: coveringIndex,
}

var tpccOrderIndex = indexDDL{
	name:        "order_idx",
	table:       "order",
	columns:     []string{"o_w_id", "o_d_id", "o_c_id", "o_id DESC"},
	typeOfIndex: uniqueIndex,
}

var tpccOrderIndexPG = indexDDL{
	name:        "order_idx",
	table:       `"order"`,
	columns:     []string{"o_w_id", "o_d_id", "o_c_id", "o_id"},
	typeOfIndex: uniqueIndex,
}

var tpccHistorySchemaFkSuffix = []indexDDL{
	tpccHistoryCustomerFKIndex,
	tpccHistoryDistrictFKIndex,
}

var tpccStockSchemaIndex = indexDDL{
	name:        "stock_item_fk_idx",
	table:       "stock",
	columns:     []string{"s_i_id"},
	typeOfIndex: coveringIndex,
}

var tpccStockSchemaFkSuffix = []indexDDL{tpccStockSchemaIndex}

var tpccOrderLineIdx = indexDDL{
	name:        "order_line_stock_fk_idx",
	table:       "order_line",
	columns:     []string{"ol_supply_w_id", "ol_i_id"},
	typeOfIndex: coveringIndex,
}

var tpccOrderLineSchemaFkSuffix = []indexDDL{tpccOrderLineIdx}

func addIndexes(w *tpcc, base string, indexes []indexDDL) string {
	const endSchema = "\n\t);\n"
	if w.usePostgres {

		base += endSchema

		for _, index := range indexes {
			base += index.createIndexStatement()
		}

	} else {

		for _, index := range indexes {
			base += ",\n\t" + index.inlineIndex()
		}
		base += endSchema
	}

	return base
}

func maybeAddFkSuffix(w *tpcc, base string, indexes []indexDDL) string {
	const endSchema = "\n\t)"
	if !w.fks {
		return base + endSchema
	}
	return addIndexes(w, base, indexes)
}

func maybeAddInterleaveSuffix(interleave bool, base, suffix string) string {
	if !interleave {
		return base
	}
	return base + suffix
}

func scatterRanges(db *gosql.DB) error {
	tables := []string{
		`customer`,
		`district`,
		`history`,
		`item`,
		`new_order`,
		`"order"`,
		`order_line`,
		`stock`,
		`warehouse`,
	}

	var g errgroup.Group
	for _, table := range tables {
		g.Go(func() error {
			sql := fmt.Sprintf(`ALTER TABLE %s SCATTER`, table)
			if _, err := db.Exec(sql); err != nil {
				return errors.Wrapf(err, "Couldn't exec %q", sql)
			}
			return nil
		})
	}
	return g.Wait()
}
