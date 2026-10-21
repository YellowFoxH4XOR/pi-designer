package store

// OutcomeCursor is a high-water mark captured before querying outcomes. A
// completion racing a poll must never be skipped by advancing past it.
func (s *Store) OutcomeCursor() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var seq int64
	err := s.db.QueryRow(`SELECT COALESCE(MAX(seq),0) FROM run_events`).Scan(&seq)
	return seq, err
}

type Outcome struct {
	Seq   int64
	RunID string
	Type  string
}

func (s *Store) Outcomes(after, through int64) ([]Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT seq,run_id,json_extract(body,'$.type') FROM run_events WHERE seq>? AND seq<=? AND json_extract(body,'$.type') IN ('attention','done','run_failed','canceled') ORDER BY seq LIMIT 500`, after, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Outcome{}
	for rows.Next() {
		var o Outcome
		if err = rows.Scan(&o.Seq, &o.RunID, &o.Type); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
