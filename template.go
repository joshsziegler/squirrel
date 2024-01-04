package main

import (
	"fmt"
	"io"
	"strings"
)

// ShortWriter allows simplified use of fmt.Fprintln() and fmt.Fprintf with an
// io.Writer to make the code easier to read, especially large blocks.
type ShortWriter struct {
	w io.Writer
}

// N ~ Newline
func (x *ShortWriter) N(s string) {
	fmt.Fprintln(x.w, s)
}

// F ~ Format
func (x *ShortWriter) F(format string, a ...any) {
	fmt.Fprintf(x.w, format, a...)
}

// TableToGo converts a Table to its Go-ORM layer.
func TableToGo(w ShortWriter, t *Table) {
	goName := ToGoName(t.Name)
	if t.Comment != "" {
		w.F("// %s %s\n", goName, t.Comment)
	}
	w.F("type %s struct {\n", goName)
	for _, c := range t.Columns {
		ColumnToGo(w, &c)
	}
	w.N(`    _exists, _deleted bool // DB Metadata `)
	w.N("}")

	w.N("// Exists in the database.")
	w.F("func (x *%s) Exists() bool {\n", goName)
	w.N("    return x._exists")
	w.N("}")

	w.N("// Deleted when this row has been marked for deletion from the database.")
	w.F("func (x *%s) Deleted() bool {", goName)
	w.N("	return x._deleted")
	w.N("}")

	// Handle Insert, Update, Upsert, Delete
	w.F("func (x *%s) Insert(ctx context.Context, dbc DB) error {\n", goName)
	w.N(`    if x._exists {`)
	w.N(`        return merry.Wrap(ErrInsertAlreadyExists)`)
	w.N(`    } else if x.deleted {`)
	w.N(`        return merry.Wrap(ErrInsertMarkedForDeletion)`)
	w.N(`    }`)
	w.N("    res, err := dbc.NamedExecContext(ctx, `")
	w.F("        INSERT INTO %s (", t.Name)
	for i, col := range t.Columns {
		if i > 0 {
			w.F(", %s", col.Name)
		} else {
			w.F("%s", col.Name)
		}
	}
	w.N(")")
	w.F("        VALUES (")
	for i, col := range t.Columns {
		if i > 0 {
			w.F(", :%s", col.Name)
		} else {
			w.F(":%s", col.Name)
		}
	}
	w.N(")`, x)")
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	w.N("id, err := res.LastInsertId()")
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	w.N("x.ID = id")
	w.N("x._exists = true")
	w.N("return nil")
	w.N(`}`)

	w.N("// Update this row in the database.")
	w.F("func (x *%s) Update(ctx context.Context, dbc DB) error {\n", goName)
	w.N("switch {")
	w.N("case !x._exists: // doesn't exist")
	w.N("	return merry.Wrap(ErrUpdateDoesNotExist)")
	w.N("case x._deleted: // deleted")
	w.N("	return ErrUpdateMarkedForDeletion")
	w.N("}")
	w.N("// update with primary key")
	w.N("_, err := dbc.NamedExecContext(ctx, `")
	w.F("        UPDATE %s (", t.Name)
	for i, col := range t.Columns { // TODO: Exclude DB-generated fields
		if col.PrimaryKey {
			continue // skip
		}
		if i > 0 {
			w.F(", %s", col.Name)
		} else {
			w.F("%s", col.Name)
		}
	}
	w.N(")")
	w.F("        VALUES (")
	for i, col := range t.Columns { // TODO: Exclude DB-generated fields
		if col.PrimaryKey {
			continue // skip
		}
		if i > 0 {
			w.F(", :%s", col.Name)
		} else {
			w.F(":%s", col.Name)
		}
	}
	w.N(")")
	w.N("        WHERE id = :id`, x)") // TODO: How to determine PK, or composite PK?
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	w.N("return nil")
	w.N("}")

	w.N("// Save this row to the database, either using Insert or Update.")
	w.F("func (x *%s) Save(ctx context.Context, dbc DB) error {", goName)
	w.N("    if x.Exists() {")
	w.N("        return x.Update(ctx, dbc)")
	w.N("    }")
	w.N("    return x.Insert(ctx, dbc)")
	w.N("}")

	// TODO: Add UPSERT

	w.N("// Delete this row from the database.")
	w.N("func (x *LoginAttempt) Delete(ctx context.Context, dbc DB) error {")
	w.N("switch {")
	w.N("case !x._exists: // doesn't exist")
	w.N("	return nil")
	w.N("case x._deleted:")
	w.N("	return nil")
	w.N("}")
	w.N("_, err := dbc.NamedExecContext(ctx, `")
	w.F("        DELETE FROM %s\n", t.Name)
	w.F("        WHERE id = :id`, x)\n") // TODO: How to determine PK, or composite PK?
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	w.N("x._deleted = true")
	w.N("return nil")
	w.N("}")
}

// ColumnToGo converts a Column to its Go-ORM layer.
// Adds a comment about whether this column is Unique, has a Default, and the SQL comment
func ColumnToGo(w ShortWriter, c *Column) {
	commentParts := []string{}
	if c.ForeignKey != nil {
		commentParts = append(commentParts, fmt.Sprintf("FK: %s.%s", c.ForeignKey.Table, c.ForeignKey.ColumnName))
	}
	if c.Unique {
		commentParts = append(commentParts, "Unique")
	}
	switch {
	case c.DefaultBool.Valid:
		commentParts = append(commentParts, fmt.Sprintf("Default: %v", c.DefaultBool.Bool))
	case c.DefaultString.Valid:
		commentParts = append(commentParts, fmt.Sprintf("Default: %v", c.DefaultString.String))
	case c.DefaultInt.Valid:
		commentParts = append(commentParts, fmt.Sprintf("Default: %v", c.DefaultInt.Int64))
	}
	//if c.Comment != "" {
	//	commentParts = append(commentParts, "--", c.Comment)
	//}
	comment := ""
	if len(commentParts) > 0 {
		comment = fmt.Sprintf("// %s", strings.Join(commentParts, ", "))
	}

	w.F("    %s %s %s\n", ToGoName(c.Name), c.Type.ToGo(c.Nullable), comment)
}
