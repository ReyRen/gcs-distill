package postgres

import "database/sql"

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}

	return ""
}