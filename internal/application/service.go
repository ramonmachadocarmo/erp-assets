package application

import (
	"context"
	"time"

	"erp/pkg/codes"
	"erp/services/assets-service/internal/domain"
)

type Service struct {
	repo    domain.Repository
	catalog domain.Catalog
	tagSeq  codes.Sequence
}

func New(repo domain.Repository, catalog domain.Catalog, tagSeq codes.Sequence) *Service {
	return &Service{repo: repo, catalog: catalog, tagSeq: tagSeq}
}

func (s *Service) Create(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	if a.DepreciationMethod == "" {
		a.DepreciationMethod = domain.MethodStraightLine
	}
	tag, err := codes.Assign(ctx, a.Tag, s.tagSeq)
	if err != nil {
		return domain.Asset{}, err
	}
	a.Tag = tag
	if err := domain.Validate(a); err != nil {
		return domain.Asset{}, err
	}
	if err := s.catalog.EnsureFixedAssetProduct(ctx, a.ProductID); err != nil {
		return domain.Asset{}, err
	}
	a.Status = domain.StatusActive
	a.AccumulatedDepreciation = 0
	a = a.WithBook()
	m := domain.Movement{
		MovementType: domain.MoveAcquire,
		ToLocation:   a.Location,
		Amount:       a.AcquisitionCost,
		Notes:        "Aquisição",
		OccurredAt:   a.AcquisitionDate,
	}
	out, err := s.repo.Create(ctx, a, m)
	if err != nil {
		return domain.Asset{}, err
	}
	return out.WithBook(), nil
}

func (s *Service) Update(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	cur, err := s.repo.Get(ctx, a.ID)
	if err != nil {
		return domain.Asset{}, err
	}
	if cur.Status != domain.StatusActive {
		return domain.Asset{}, domain.ErrDisposed
	}
	cur.Tag = a.Tag
	cur.SerialNumber = a.SerialNumber
	cur.Description = a.Description
	cur.Location = a.Location
	cur.AcquisitionDate = a.AcquisitionDate
	cur.AcquisitionCost = a.AcquisitionCost
	cur.ResidualValue = a.ResidualValue
	cur.UsefulLifeMonths = a.UsefulLifeMonths
	if a.DepreciationMethod != "" {
		cur.DepreciationMethod = a.DepreciationMethod
	}
	if err := domain.Validate(cur); err != nil {
		return domain.Asset{}, err
	}
	if err := s.repo.Update(ctx, cur); err != nil {
		return domain.Asset{}, err
	}
	return cur.WithBook(), nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Asset, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Asset{}, err
	}
	mov, err := s.repo.ListMovements(ctx, id)
	if err != nil {
		return domain.Asset{}, err
	}
	a.Movements = mov
	return a.WithBook(), nil
}

func (s *Service) List(ctx context.Context) ([]domain.Asset, error) {
	out, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i] = out[i].WithBook()
	}
	return out, nil
}

func (s *Service) Transfer(ctx context.Context, id string, in domain.TransferInput) (domain.Asset, error) {
	if in.Location == "" {
		return domain.Asset{}, domain.ErrInvalid
	}
	a, err := s.requireActive(ctx, id)
	if err != nil {
		return domain.Asset{}, err
	}
	from := a.Location
	a.Location = in.Location
	m := domain.Movement{
		MovementType: domain.MoveTransfer,
		FromLocation: from,
		ToLocation:   in.Location,
		Notes:        in.Notes,
		OccurredAt:   time.Now().UTC(),
	}
	if err := s.repo.AddMovement(ctx, a, m); err != nil {
		return domain.Asset{}, err
	}
	return a.WithBook(), nil
}

func (s *Service) Depreciate(ctx context.Context, id string, asOf time.Time) (domain.Asset, error) {
	a, err := s.requireActive(ctx, id)
	if err != nil {
		return domain.Asset{}, err
	}
	return s.accrue(ctx, a, asOf)
}

func (s *Service) DepreciateAll(ctx context.Context, asOf time.Time) ([]domain.Asset, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Asset, 0, len(list))
	for _, a := range list {
		if a.Status != domain.StatusActive {
			continue
		}
		updated, err := s.accrue(ctx, a, asOf)
		if err != nil {
			return nil, err
		}
		out = append(out, updated)
	}
	return out, nil
}

func (s *Service) Dispose(ctx context.Context, id string, in domain.DisposeInput) (domain.Asset, error) {
	a, err := s.requireActive(ctx, id)
	if err != nil {
		return domain.Asset{}, err
	}
	accrued, err := s.accrue(ctx, a, time.Now().UTC())
	if err != nil {
		return domain.Asset{}, err
	}
	accrued.Status = domain.StatusDisposed
	m := domain.Movement{
		MovementType: domain.MoveDispose,
		FromLocation: accrued.Location,
		Amount:       accrued.NetBookValue,
		Notes:        in.Notes,
		OccurredAt:   time.Now().UTC(),
	}
	if err := s.repo.AddMovement(ctx, accrued, m); err != nil {
		return domain.Asset{}, err
	}
	return accrued.WithBook(), nil
}

func (s *Service) ListMovements(ctx context.Context, assetID string) ([]domain.Movement, error) {
	if assetID == "" {
		return s.repo.ListAllMovements(ctx)
	}
	return s.repo.ListMovements(ctx, assetID)
}

func (s *Service) requireActive(ctx context.Context, id string) (domain.Asset, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Asset{}, err
	}
	if a.Status != domain.StatusActive {
		return domain.Asset{}, domain.ErrDisposed
	}
	return a, nil
}

func (s *Service) accrue(ctx context.Context, a domain.Asset, asOf time.Time) (domain.Asset, error) {
	updated, delta := domain.Accrue(a, asOf)
	if delta == 0 {
		return updated, nil
	}
	m := domain.Movement{
		MovementType: domain.MoveDepreciate,
		Amount:       delta,
		Notes:        "Depreciação linear",
		OccurredAt:   asOf,
	}
	if err := s.repo.AddMovement(ctx, updated, m); err != nil {
		return domain.Asset{}, err
	}
	return updated, nil
}
