package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"finance.local/amlens/internal/domain"
	"fmt"
	"io"
	"sync"
)

type Error struct{ Code, Message string }

func (e *Error) Error() string        { return e.Message }
func Fail(code, message string) error { return &Error{code, message} }

type Reader interface {
	Read(context.Context, string) (domain.Dataset, error)
}
type Grouper interface {
	Groups(context.Context, domain.Dataset) ([][]int64, error)
}
type Exporter interface {
	Encode(domain.Analysis) (map[string][]byte, error)
}
type Upload struct {
	Name, Filename string
	Reader         io.Reader
}
type UploadSource interface{ Next() (Upload, error) }
type Files interface {
	Save(context.Context, UploadSource) (string, func(), error)
}
type Snapshot struct {
	Analysis domain.Analysis
	Exports  map[string][]byte
}
type Service struct {
	reader   Reader
	grouper  Grouper
	exporter Exporter
	files    Files
	model    Model
	busy     chan struct{}
	mu       sync.RWMutex
	current  *Snapshot
	store    SnapshotStore
}

func New(reader Reader, grouper Grouper, exporter Exporter, files Files, model Model) *Service {
	return &Service{reader: reader, grouper: grouper, exporter: exporter, files: files, model: model, busy: make(chan struct{}, 1)}
}
func (s *Service) Validate(ctx context.Context, dir string) (domain.Dataset, error) {
	d, err := s.reader.Read(ctx, dir)
	if err != nil {
		return d, Fail("INVALID_SCHEMA", err.Error())
	}
	if err = domain.Validate(d); err != nil {
		return d, Fail("INVALID_SCHEMA", err.Error())
	}
	return d, nil
}
func (s *Service) Analyze(ctx context.Context, dir string) (*Snapshot, error) {
	d, err := s.Validate(ctx, dir)
	if err != nil {
		return nil, err
	}
	groups, err := s.grouper.Groups(ctx, d)
	if err != nil {
		return nil, err
	}
	a, err := domain.Analyze(d, groups)
	if err != nil {
		return nil, Fail("INVALID_SCHEMA", err.Error())
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return nil, err
	}
	a.ID = hex.EncodeToString(id)
	csv, err := s.exporter.Encode(a)
	if err != nil {
		return nil, err
	}
	return &Snapshot{a, csv}, nil
}
func (s *Service) Publish(ctx context.Context, source UploadSource) (*Snapshot, error) {
	select {
	case s.busy <- struct{}{}:
	default:
		return nil, Fail("ANALYSIS_BUSY", "На сервере уже выполняется расчёт")
	}
	defer func() { <-s.busy }()
	dir, cleanup, err := s.files.Save(ctx, source)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	snapshot, err := s.Analyze(ctx, dir)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if s.store != nil {
		if err = s.store.Save(ctx, snapshot); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	s.current = snapshot
	s.mu.Unlock()
	return snapshot, nil
}

// Snapshots and their byte slices are immutable after publication.
func (s *Service) Current() (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return nil, Fail("NO_ANALYSIS", "Сначала загрузите три файла и выполните анализ")
	}
	return s.current, nil
}
func (s *Service) Card(gid int64) (domain.Card, error) {
	snap, err := s.Current()
	if err != nil {
		return domain.Card{}, err
	}
	for _, n := range snap.Analysis.Nodes {
		if n.GID == gid {
			return domain.NodeCard(snap.Analysis, n), nil
		}
	}
	return domain.Card{}, Fail("GID_NOT_FOUND", "Узел не найден в текущем анализе")
}
func (s *Service) Export(name string) ([]byte, string, error) {
	if name != "nodes_roles.csv" && name != "clusters.csv" && name != "top_nodes.csv" {
		return nil, "", Fail("EXPORT_NOT_FOUND", "Файл экспорта не найден")
	}
	snap, err := s.Current()
	if err != nil {
		return nil, "", err
	}
	data, ok := snap.Exports[name]
	if !ok {
		return nil, "", fmt.Errorf("export is missing")
	}
	return data, snap.Analysis.ID, nil
}
func (s *Service) AIConfigured() bool { return s.model != nil && s.model.Configured() }
