package application

import "context"

type AnalysisRecord struct {
	ID        string `json:"analysis_id"`
	CreatedAt string `json:"created_at"`
	Nodes     int    `json:"n_nodes"`
	Edges     int    `json:"n_edges"`
}

type SnapshotStore interface {
	Save(context.Context, *Snapshot) error
	Current(context.Context) (*Snapshot, error)
	List(context.Context) ([]AnalysisRecord, error)
	Activate(context.Context, string) (*Snapshot, error)
}

// AttachStore is called once, before accepting requests. A corrupt store must
// fail startup rather than silently discard previously saved work.
func (s *Service) AttachStore(ctx context.Context, store SnapshotStore) error {
	snapshot, err := store.Current(ctx)
	if err != nil {
		return err
	}
	s.store, s.current = store, snapshot
	return nil
}

func (s *Service) History(ctx context.Context) ([]AnalysisRecord, error) {
	if s.store == nil {
		return []AnalysisRecord{}, nil
	}
	return s.store.List(ctx)
}

func (s *Service) Activate(ctx context.Context, id string) (*Snapshot, error) {
	select {
	case s.busy <- struct{}{}:
	default:
		return nil, Fail("ANALYSIS_BUSY", "На сервере уже выполняется расчёт")
	}
	defer func() { <-s.busy }()
	if s.store == nil {
		return nil, Fail("ANALYSIS_NOT_FOUND", "Сохранённый анализ не найден")
	}
	snapshot, err := s.store.Activate(ctx, id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.current = snapshot
	s.mu.Unlock()
	return snapshot, nil
}

// Seed initializes an empty demo store before the HTTP server starts.
func (s *Service) Seed(ctx context.Context, dir string) error {
	if s.current != nil {
		return nil
	}
	snapshot, err := s.Analyze(ctx, dir)
	if err != nil {
		return err
	}
	if s.store != nil {
		if err = s.store.Save(ctx, snapshot); err != nil {
			return err
		}
	}
	s.current = snapshot
	return nil
}
