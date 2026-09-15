package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalid       = errors.New("invalid")
	ErrNotFixedAsset = errors.New("product is not a fixed asset")
	ErrDisposed      = errors.New("asset disposed")
	ErrConflict      = errors.New("already exists")
)

const (
	StatusActive   = "ACTIVE"
	StatusDisposed = "DISPOSED"

	MethodStraightLine = "STRAIGHT_LINE"

	MoveAcquire    = "ACQUIRE"
	MoveTransfer   = "TRANSFER"
	MoveDepreciate = "DEPRECIATE"
	MoveDispose    = "DISPOSE"
)

type Asset struct {
	ID                       string     `json:"id"`
	ProductID                string     `json:"product_id"`
	Tag                      string     `json:"tag"`
	SerialNumber             string     `json:"serial_number"`
	Description              string     `json:"description"`
	Location                 string     `json:"location"`
	AcquisitionDate          time.Time  `json:"acquisition_date"`
	AcquisitionCost          float64    `json:"acquisition_cost"`
	ResidualValue            float64    `json:"residual_value"`
	UsefulLifeMonths         int        `json:"useful_life_months"`
	DepreciationMethod       string     `json:"depreciation_method"`
	AccumulatedDepreciation  float64    `json:"accumulated_depreciation"`
	NetBookValue             float64    `json:"net_book_value"`
	Status                   string     `json:"status"`
	CreatedAt                time.Time  `json:"created_at"`
	Movements                []Movement `json:"movements,omitempty"`
}

type Movement struct {
	ID           string    `json:"id"`
	AssetID      string    `json:"asset_id"`
	MovementType string    `json:"movement_type"`
	FromLocation string    `json:"from_location"`
	ToLocation   string    `json:"to_location"`
	Amount       float64   `json:"amount"`
	Notes        string    `json:"notes"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type TransferInput struct {
	Location string `json:"location"`
	Notes    string `json:"notes"`
}

type DisposeInput struct {
	Notes string `json:"notes"`
}

type DepreciateInput struct {
	AsOf *time.Time `json:"as_of"`
}

type Catalog interface {
	EnsureFixedAssetProduct(ctx context.Context, productID string) error
}

type Repository interface {
	Create(ctx context.Context, a Asset, m Movement) (Asset, error)
	Update(ctx context.Context, a Asset) error
	Get(ctx context.Context, id string) (Asset, error)
	List(ctx context.Context) ([]Asset, error)
	AddMovement(ctx context.Context, a Asset, m Movement) error
	ListMovements(ctx context.Context, assetID string) ([]Movement, error)
	ListAllMovements(ctx context.Context) ([]Movement, error)
}

func (a Asset) WithBook() Asset {
	a.NetBookValue = a.AcquisitionCost - a.AccumulatedDepreciation
	if a.NetBookValue < 0 {
		a.NetBookValue = 0
	}
	return a
}

func MonthlyDepreciation(a Asset) float64 {
	depreciable := a.AcquisitionCost - a.ResidualValue
	if a.UsefulLifeMonths <= 0 || depreciable <= 0 {
		return 0
	}
	return depreciable / float64(a.UsefulLifeMonths)
}

func MonthsBetween(from, to time.Time) int {
	from = dateOnly(from)
	to = dateOnly(to)
	if to.Before(from) {
		return 0
	}
	y, m, d := to.Date()
	fy, fm, fd := from.Date()
	n := (y-fy)*12 + int(m-fm)
	if d < fd {
		n--
	}
	if n < 0 {
		return 0
	}
	return n
}

func AccruedUntil(a Asset, asOf time.Time) float64 {
	months := MonthsBetween(a.AcquisitionDate, asOf)
	if months > a.UsefulLifeMonths {
		months = a.UsefulLifeMonths
	}
	max := a.AcquisitionCost - a.ResidualValue
	if max < 0 {
		max = 0
	}
	v := MonthlyDepreciation(a) * float64(months)
	if v > max {
		return max
	}
	return v
}

func Accrue(a Asset, asOf time.Time) (Asset, float64) {
	target := AccruedUntil(a, asOf)
	delta := target - a.AccumulatedDepreciation
	if delta < 0 {
		delta = 0
	}
	a.AccumulatedDepreciation = target
	return a.WithBook(), delta
}

func Validate(a Asset) error {
	if a.ProductID == "" || a.Tag == "" {
		return ErrInvalid
	}
	if a.AcquisitionCost <= 0 || a.ResidualValue < 0 || a.ResidualValue >= a.AcquisitionCost {
		return ErrInvalid
	}
	if a.UsefulLifeMonths <= 0 {
		return ErrInvalid
	}
	if a.DepreciationMethod != "" && a.DepreciationMethod != MethodStraightLine {
		return ErrInvalid
	}
	if a.AcquisitionDate.IsZero() {
		return ErrInvalid
	}
	return nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
