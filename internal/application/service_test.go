package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"erp/services/assets-service/internal/domain"
)

type memSeq struct{ n int64 }

func (s *memSeq) Next(context.Context) (int64, error) {
	s.n++
	return s.n, nil
}
func (s *memSeq) EnsureMin(_ context.Context, n int64) error {
	if n > s.n {
		s.n = n
	}
	return nil
}

type okCat struct{}

func (okCat) EnsureFixedAssetProduct(context.Context, string) error { return nil }

type badCat struct{}

func (badCat) EnsureFixedAssetProduct(context.Context, string) error { return domain.ErrNotFixedAsset }

type memAssets struct {
	byID map[string]domain.Asset
	mov  []domain.Movement
	n    int
}

func (m *memAssets) Create(_ context.Context, a domain.Asset, mv domain.Movement) (domain.Asset, error) {
	m.n++
	a.ID = fmt.Sprintf("a%d", m.n)
	if m.byID == nil {
		m.byID = map[string]domain.Asset{}
	}
	m.byID[a.ID] = a
	mv.AssetID = a.ID
	m.mov = append(m.mov, mv)
	return a, nil
}

func (m *memAssets) Update(_ context.Context, a domain.Asset) error {
	if _, ok := m.byID[a.ID]; !ok {
		return domain.ErrNotFound
	}
	m.byID[a.ID] = a
	return nil
}

func (m *memAssets) Get(_ context.Context, id string) (domain.Asset, error) {
	a, ok := m.byID[id]
	if !ok {
		return domain.Asset{}, domain.ErrNotFound
	}
	return a, nil
}

func (m *memAssets) List(context.Context) ([]domain.Asset, error) {
	out := make([]domain.Asset, 0, len(m.byID))
	for _, a := range m.byID {
		out = append(out, a)
	}
	return out, nil
}

func (m *memAssets) AddMovement(_ context.Context, a domain.Asset, mv domain.Movement) error {
	if _, ok := m.byID[a.ID]; !ok {
		return domain.ErrNotFound
	}
	m.byID[a.ID] = a
	mv.AssetID = a.ID
	m.mov = append(m.mov, mv)
	return nil
}

func (m *memAssets) ListMovements(_ context.Context, assetID string) ([]domain.Movement, error) {
	var out []domain.Movement
	for _, mv := range m.mov {
		if mv.AssetID == assetID {
			out = append(out, mv)
		}
	}
	return out, nil
}

func (m *memAssets) ListAllMovements(context.Context) ([]domain.Movement, error) {
	return m.mov, nil
}

func sample() domain.Asset {
	return domain.Asset{
		ProductID: "p1", Tag: "AF-1", Location: "CD",
		AcquisitionCost: 12000, ResidualValue: 0, UsefulLifeMonths: 12,
		AcquisitionDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	}
}

func TestCreate(t *testing.T) {
	repo := &memAssets{}
	got, err := New(repo, okCat{}, &memSeq{}).Create(context.Background(), sample())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusActive || got.Tag != "AF-1" || got.NetBookValue != 12000 {
		t.Fatalf("%+v", got)
	}
	if len(repo.mov) != 1 || repo.mov[0].MovementType != domain.MoveAcquire {
		t.Fatalf("%+v", repo.mov)
	}
}

func TestCreateAssignsTag(t *testing.T) {
	a := sample()
	a.Tag = ""
	got, err := New(&memAssets{}, okCat{}, &memSeq{}).Create(context.Background(), a)
	if err != nil || got.Tag != "000001" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestCreateRejectsNonFixedAsset(t *testing.T) {
	_, err := New(&memAssets{}, badCat{}, &memSeq{}).Create(context.Background(), sample())
	if err != domain.ErrNotFixedAsset {
		t.Fatalf("%v", err)
	}
}

func TestTransferDepreciateDispose(t *testing.T) {
	repo := &memAssets{}
	svc := New(repo, okCat{}, &memSeq{})
	a, err := svc.Create(context.Background(), sample())
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Transfer(context.Background(), a.ID, domain.TransferInput{Location: "Filial"})
	if err != nil || got.Location != "Filial" {
		t.Fatalf("%v %+v", err, got)
	}
	got, err = svc.Depreciate(context.Background(), a.ID, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC))
	if err != nil || got.AccumulatedDepreciation != 6000 {
		t.Fatalf("%v %+v", err, got)
	}
	got, err = svc.Dispose(context.Background(), a.ID, domain.DisposeInput{Notes: "baixa"})
	if err != nil || got.Status != domain.StatusDisposed {
		t.Fatalf("%v %+v", err, got)
	}
	if _, err := svc.Transfer(context.Background(), a.ID, domain.TransferInput{Location: "X"}); err != domain.ErrDisposed {
		t.Fatalf("%v", err)
	}
}
