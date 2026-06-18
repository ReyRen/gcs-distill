package mysql

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func jsonBytes(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func unmarshalJSON(data []byte, target any) error {
	if len(data) == 0 || strings.EqualFold(strings.TrimSpace(string(data)), "null") {
		return nil
	}
	return json.Unmarshal(data, target)
}
