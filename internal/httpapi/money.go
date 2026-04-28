package httpapi

import (
	"encoding/json"
	"errors"
	"math"
)

// money stores value in minor units (cents).
type money int64

func (m money) Float() float64 { return float64(m) / 100.0 }
func (m money) Int64() int64   { return int64(m) }

func parseMoney(v any) (money, error) {
	switch t := v.(type) {
	case float64:
		if t < 0 {
			return 0, errors.New("negative")
		}
		return money(math.Round(t * 100)), nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0, err
		}
		if f < 0 {
			return 0, errors.New("negative")
		}
		return money(math.Round(f * 100)), nil
	case int:
		if t < 0 {
			return 0, errors.New("negative")
		}
		return money(int64(t) * 100), nil
	case int64:
		if t < 0 {
			return 0, errors.New("negative")
		}
		return money(t * 100), nil
	default:
		return 0, errors.New("unsupported sum type")
	}
}
