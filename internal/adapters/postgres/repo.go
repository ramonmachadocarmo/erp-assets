package postgres

import (
	"context"
	"errors"
	"strings"

	"erp/services/assets-service/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Create(ctx context.Context, a domain.Asset, m domain.Movement) (domain.Asset, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Asset{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO fixed_assets (
			product_id, tag, serial_number, description, location,
			acquisition_date, acquisition_cost, residual_value, useful_life_months,
			depreciation_method, accumulated_depreciation, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at
	`, a.ProductID, a.Tag, a.SerialNumber, a.Description, a.Location,
		a.AcquisitionDate, a.AcquisitionCost, a.ResidualValue, a.UsefulLifeMonths,
		a.DepreciationMethod, a.AccumulatedDepreciation, a.Status,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		if isUnique(err) {
			return domain.Asset{}, domain.ErrConflict
		}
		return domain.Asset{}, err
	}
	m.AssetID = a.ID
	if err := insertMovement(ctx, tx, m); err != nil {
		return domain.Asset{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Asset{}, err
	}
	return a, nil
}

func (r *Repo) Update(ctx context.Context, a domain.Asset) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE fixed_assets SET
			tag=$2, serial_number=$3, description=$4, location=$5,
			acquisition_date=$6, acquisition_cost=$7, residual_value=$8,
			useful_life_months=$9, depreciation_method=$10,
			accumulated_depreciation=$11, status=$12
		WHERE id=$1
	`, a.ID, a.Tag, a.SerialNumber, a.Description, a.Location,
		a.AcquisitionDate, a.AcquisitionCost, a.ResidualValue, a.UsefulLifeMonths,
		a.DepreciationMethod, a.AccumulatedDepreciation, a.Status)
	if err != nil {
		if isUnique(err) {
			return domain.ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) Get(ctx context.Context, id string) (domain.Asset, error) {
	a, err := scanAsset(r.pool.QueryRow(ctx, assetSelect+" WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Asset{}, domain.ErrNotFound
	}
	return a, err
}

func (r *Repo) List(ctx context.Context) ([]domain.Asset, error) {
	rows, err := r.pool.Query(ctx, assetSelect+" ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []domain.Asset{}
	}
	return out, rows.Err()
}

func (r *Repo) AddMovement(ctx context.Context, a domain.Asset, m domain.Movement) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE fixed_assets SET
			location=$2, accumulated_depreciation=$3, status=$4
		WHERE id=$1
	`, a.ID, a.Location, a.AccumulatedDepreciation, a.Status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	m.AssetID = a.ID
	if err := insertMovement(ctx, tx, m); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repo) ListMovements(ctx context.Context, assetID string) ([]domain.Movement, error) {
	return r.queryMovements(ctx, movementSelect+" WHERE asset_id=$1 ORDER BY occurred_at DESC", assetID)
}

func (r *Repo) ListAllMovements(ctx context.Context) ([]domain.Movement, error) {
	return r.queryMovements(ctx, movementSelect+" ORDER BY occurred_at DESC")
}

func (r *Repo) queryMovements(ctx context.Context, q string, args ...any) ([]domain.Movement, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Movement
	for rows.Next() {
		var m domain.Movement
		if err := rows.Scan(&m.ID, &m.AssetID, &m.MovementType, &m.FromLocation, &m.ToLocation, &m.Amount, &m.Notes, &m.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []domain.Movement{}
	}
	return out, rows.Err()
}

const assetSelect = `
	SELECT id, product_id, tag, serial_number, description, location,
		acquisition_date, acquisition_cost, residual_value, useful_life_months,
		depreciation_method, accumulated_depreciation, status, created_at
	FROM fixed_assets`

const movementSelect = `
	SELECT id, asset_id, movement_type, from_location, to_location, amount, notes, occurred_at
	FROM asset_movements`

type scanner interface {
	Scan(dest ...any) error
}

func scanAsset(s scanner) (domain.Asset, error) {
	var a domain.Asset
	err := s.Scan(
		&a.ID, &a.ProductID, &a.Tag, &a.SerialNumber, &a.Description, &a.Location,
		&a.AcquisitionDate, &a.AcquisitionCost, &a.ResidualValue, &a.UsefulLifeMonths,
		&a.DepreciationMethod, &a.AccumulatedDepreciation, &a.Status, &a.CreatedAt,
	)
	return a, err
}

type execer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertMovement(ctx context.Context, tx execer, m domain.Movement) error {
	return tx.QueryRow(ctx, `
		INSERT INTO asset_movements (asset_id, movement_type, from_location, to_location, amount, notes, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id
	`, m.AssetID, m.MovementType, m.FromLocation, m.ToLocation, m.Amount, m.Notes, m.OccurredAt).Scan(&m.ID)
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23505" || strings.Contains(err.Error(), "duplicate"))
}
