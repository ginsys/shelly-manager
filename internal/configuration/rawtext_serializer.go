package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"gorm.io/gorm/schema"
)

// rawTextSerializer stores a json.RawMessage in a text column as text and reads it back from the
// string or bytes a driver returns. Without it PostgreSQL's string for a text column cannot be
// scanned into a RawMessage. A write must be valid JSON; a read hands back whatever the column holds,
// so one corrupt row fails its own run and does not make every schedule unreadable.
type rawTextSerializer struct{}

func init() { schema.RegisterSerializer("rawtext", rawTextSerializer{}) }

func (rawTextSerializer) Scan(_ context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	var raw json.RawMessage
	switch v := dbValue.(type) {
	case nil:
	case []byte:
		raw = append(json.RawMessage{}, v...)
	case string:
		raw = json.RawMessage(v)
	default:
		return fmt.Errorf("scan %T into a raw JSON text column", dbValue)
	}
	return field.Set(context.Background(), dst, raw)
}

func (rawTextSerializer) Value(_ context.Context, _ *schema.Field, _ reflect.Value, fieldValue any) (any, error) {
	var raw json.RawMessage
	switch v := fieldValue.(type) {
	case json.RawMessage:
		raw = v
	case []byte:
		raw = v
	case string:
		raw = json.RawMessage(v)
	case nil:
	default:
		return nil, fmt.Errorf("store %T in a raw JSON text column", fieldValue)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if !json.Valid(raw) {
		return nil, errors.New("the value is not valid JSON")
	}
	return string(raw), nil
}
