// SPDX-License-Identifier: AGPL-3.0-only
package retention

import (
	"context"
	"database/sql"
)

func DeleteBefore(ctx context.Context, db *sql.DB, table string, cutoff int64, limit int) (int64, error) {
	if limit < 1 {
		limit = 500
	}
	r, e := db.ExecContext(ctx, "DELETE FROM "+table+" WHERE rowid IN (SELECT rowid FROM "+table+" WHERE ts<? LIMIT ?)", cutoff, limit)
	if e != nil {
		return 0, e
	}
	return r.RowsAffected()
}
