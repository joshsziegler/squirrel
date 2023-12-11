package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrep(t *testing.T) {
	exampleA := `CREATE TABLE foo ( bar TEXT )`
	exampleB := `CREATE TABLE "foo" ( bar TEXT )`
	exampleC := `CREATE TABLE foo (
		bar TEXT
	)
	`

	// assert.True(t, createTable.Match(exampleA))
	// assert.True(t, createTable.Match(exampleB))
	// assert.True(t, createTable.Match(exampleC))

	got := grep(exampleA)
	got = grep(exampleB)
	got = grep(exampleC)
	assert.Equal(t, []string{"CREATE TABLE foo ("}, got)

}

func TestParse(t *testing.T) {

}
