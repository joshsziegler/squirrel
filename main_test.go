package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr bool
		want    []*Table
	}{
		{"basic table",
			`CREATE TABLE users ( name TEXT PRIMARY KEY )`, false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: true},
					},
				},
			},
		},
		{"basic table without nullable primary key",
			`CREATE TABLE users ( name TEXT PRIMARY KEY NOT NULL)`, false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: false},
					},
				},
			},
		},
		{"basic table with numeric ID primary key, but nullable unique name",
			`CREATE TABLE users (
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT,
				name TEXT UNIQUE
			)`,
			false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "name", Type: "string", PrimaryKey: false, Nullable: true, Unique: true},
					},
				},
			},
		},
		{"basic table with integer number of nodes and semicolon at end of definition",
			`CREATE TABLE jobs (
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT,
				num_nodes INTEGER
			);`,
			false,
			[]*Table{
				{
					Name: "jobs",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "num_nodes", Type: "int64", PrimaryKey: false, Nullable: true},
					},
				},
			},
		},
		{"basic table with quotes table name and comments at end of the CREATE TABLE line",
			`CREATE TABLE "foo" ( -- Hello world!
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT
			);`,
			false,
			[]*Table{
				{
					Name: "foo",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
					},
				},
			},
		},
		{"basic table with comments at end of a column line",
			`CREATE TABLE bars (
				name TEXT NOT NULL UNIQUE -- name of the bar
			);`,
			false,
			[]*Table{
				{
					Name: "bars",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: false, Nullable: false, Unique: true},
					},
				},
			},
		},
		{"basic table with every data type supported by STRICT",
			`CREATE TABLE animals (
				name TEXT PRIMARY KEY NOT NULL,
				age INT NOT NULL,
				weight REAL NOT NULL,
				height INTEGER NOT NULL,
				photo BLOB,
				data ANY
			);`,
			false,
			[]*Table{
				{
					Name: "animals",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: false},
						{Name: "age", Type: "int64", Nullable: false},
						{Name: "weight", Type: "float", Nullable: false},
						{Name: "height", Type: "int64", Nullable: false},
						{Name: "photo", Type: "byte", Nullable: true},
						{Name: "data", Type: "byte", Nullable: true},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.sql)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "%+v", got)
		})
	}
}
