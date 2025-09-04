package mss_test

import (
	"testing"

	"github.com/Ret-Samphis/mss"
)

// More tests to come
func TestMixedStructSlice_GetRowMut(t *testing.T) {
	type Position struct {
		X, Y, Z float64
	}

	type Velocity struct {
		X, Y, Z float64
	}
	testSlice := mss.MixedStructSlice{}
	testSlice.AddComponent(Position{})
	testSlice.AddComponent(Velocity{})
	testSlice.Build()
	testSlice.Add(Position{1, 2, 3}, Velocity{4, 5, 6})
	out := mss.NewRowViewMut(&testSlice)
	out2 := mss.NewRowViewCopy(&testSlice)
	out.SetIndex(0)
	out2.SetIndex(0)
	outPos := mss.RowGet[Position](out, 0)
	outVel := mss.RowGet[Velocity](out, 1)
	outPos2 := mss.RowGetCopy[Position](out2, 0)
	outVel2 := mss.RowGetCopy[Velocity](out2, 1)
	outPos.X += 10
	outVel.Y += 30
	_, _ = outPos2, outVel2
}
