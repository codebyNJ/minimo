package opencode

import (
	"database/sql"
	"time"
)

type sessionRow struct {
	id               string
	directory        string
	title            string
	model            string
	cost             float64
	tokensInput      int64
	tokensOutput     int64
	tokensReasoning  int64
	tokensCacheRead  int64
	tokensCacheWrite int64
	timeCreated      int64
	timeUpdated      int64
	timeArchived     sql.NullInt64
}

const sessionColumns = `id, directory, title, model, cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write, time_created, time_updated, time_archived`

func scanSessionRow(scan func(...any) error) (sessionRow, error) {
	var r sessionRow
	err := scan(&r.id, &r.directory, &r.title, &r.model, &r.cost, &r.tokensInput, &r.tokensOutput,
		&r.tokensReasoning, &r.tokensCacheRead, &r.tokensCacheWrite,
		&r.timeCreated, &r.timeUpdated, &r.timeArchived)
	return r, err
}

func listSessions(db *sql.DB) ([]sessionRow, error) {
	rows, err := db.Query(`SELECT ` + sessionColumns + ` FROM session`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []sessionRow
	for rows.Next() {
		r, err := scanSessionRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func readSession(db *sql.DB, id string) (*sessionRow, error) {
	row := db.QueryRow(`SELECT `+sessionColumns+` FROM session WHERE id = ?`, id)
	r, err := scanSessionRow(row.Scan)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func epochMillis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
