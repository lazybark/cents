package sqlite

import (
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Rows that records link to (categories, accounts, payment methods, tax
// types) have a UID. registerUIDs makes every new one get a UUID however it
// is created, and keeps a save of a row whose UID wasn't loaded (a record
// built from a form, say) from blanking it.
func registerUIDs(db *gorm.DB) error {
	if err := db.Callback().Create().Before("gorm:create").Register("cents:assign_uid", assignUIDs); err != nil {
		return err
	}

	return db.Callback().Update().Before("gorm:update").Register("cents:keep_uid", keepUIDs)
}

func assignUIDs(tx *gorm.DB) {
	if tx.Statement.Schema == nil {
		return
	}

	field := tx.Statement.Schema.LookUpField("UID")
	if field == nil {
		return
	}

	ctx := tx.Statement.Context
	set := func(v reflect.Value) {
		if _, zero := field.ValueOf(ctx, v); zero {
			_ = field.Set(ctx, v, uuid.NewString())
		}
	}

	switch rv := reflect.Indirect(tx.Statement.ReflectValue); rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			set(reflect.Indirect(rv.Index(i)))
		}
	case reflect.Struct:
		set(rv)
	}
}

func keepUIDs(tx *gorm.DB) {
	if tx.Statement.Schema == nil {
		return
	}

	field := tx.Statement.Schema.LookUpField("UID")
	if field == nil {
		return
	}

	if rv := reflect.Indirect(tx.Statement.ReflectValue); rv.Kind() == reflect.Struct {
		if _, zero := field.ValueOf(tx.Statement.Context, rv); zero {
			tx.Statement.Omits = append(tx.Statement.Omits, field.DBName)
		}
	}
}
