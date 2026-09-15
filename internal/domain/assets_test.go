package domain

import (
	"testing"
	"time"
)

func TestAccrueStraightLine(t *testing.T) {
	a := Asset{
		AcquisitionDate:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		AcquisitionCost:    12000,
		ResidualValue:      0,
		UsefulLifeMonths:   12,
		DepreciationMethod: MethodStraightLine,
	}
	got, delta := Accrue(a, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC))
	if delta != 6000 {
		t.Fatalf("delta %v", delta)
	}
	if got.AccumulatedDepreciation != 6000 || got.NetBookValue != 6000 {
		t.Fatalf("got %+v", got)
	}
	got, delta = Accrue(got, time.Date(2027, 2, 15, 0, 0, 0, 0, time.UTC))
	if delta != 6000 || got.AccumulatedDepreciation != 12000 || got.NetBookValue != 0 {
		t.Fatalf("full %+v delta %v", got, delta)
	}
}

func TestValidate(t *testing.T) {
	a := Asset{ProductID: "p", Tag: "AF-1", AcquisitionCost: 100, ResidualValue: 10, UsefulLifeMonths: 60, AcquisitionDate: time.Now()}
	if err := Validate(a); err != nil {
		t.Fatal(err)
	}
	a.ResidualValue = 100
	if err := Validate(a); err != ErrInvalid {
		t.Fatalf("want invalid, got %v", err)
	}
}
