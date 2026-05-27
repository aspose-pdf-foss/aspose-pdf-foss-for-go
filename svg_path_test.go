// SPDX-License-Identifier: MIT

package asposepdf

import (
	"math"
	"testing"
)

func TestParseSVGPathData_MoveToLineTo(t *testing.T) {
	ops, err := parseSVGPathData("M 10 20 L 30 40")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("len=%d, want 2; ops=%+v", len(ops), ops)
	}
	if ops[0].kind != 'M' || ops[0].args[0] != 10 || ops[0].args[1] != 20 {
		t.Errorf("op[0] = %+v", ops[0])
	}
	if ops[1].kind != 'L' || ops[1].args[0] != 30 || ops[1].args[1] != 40 {
		t.Errorf("op[1] = %+v", ops[1])
	}
}

func TestParseSVGPathData_ImplicitLineTo(t *testing.T) {
	ops, _ := parseSVGPathData("M 10 20 30 40")
	if len(ops) != 2 || ops[1].kind != 'L' {
		t.Errorf("expected M then implicit L, got %+v", ops)
	}
}

func TestParseSVGPathData_RelativeMovingPoint(t *testing.T) {
	ops, _ := parseSVGPathData("M 10 10 l 5 5")
	if ops[1].kind != 'L' || ops[1].args[0] != 15 || ops[1].args[1] != 15 {
		t.Errorf("relative L not resolved to absolute: %+v", ops)
	}
}

func TestParseSVGPathData_HorizontalLine(t *testing.T) {
	ops, _ := parseSVGPathData("M 0 5 H 10")
	if ops[1].kind != 'L' || ops[1].args[0] != 10 || ops[1].args[1] != 5 {
		t.Errorf("H not normalized to L: %+v", ops[1])
	}
}

func TestParseSVGPathData_VerticalLine(t *testing.T) {
	ops, _ := parseSVGPathData("M 5 0 V 10")
	if ops[1].kind != 'L' || ops[1].args[0] != 5 || ops[1].args[1] != 10 {
		t.Errorf("V not normalized to L: %+v", ops[1])
	}
}

func TestParseSVGPathData_CubicBezier(t *testing.T) {
	ops, _ := parseSVGPathData("M 0 0 C 1 2 3 4 5 6")
	if ops[1].kind != 'C' {
		t.Fatalf("kind=%c", ops[1].kind)
	}
	if ops[1].args[0] != 1 || ops[1].args[5] != 6 {
		t.Errorf("C args = %v", ops[1].args[:6])
	}
}

func TestParseSVGPathData_SmoothCubic(t *testing.T) {
	// M0,0 C1,2,3,4,5,6 S 9 10 11 12
	// S becomes C with reflected C2 from previous C as new C1.
	// previous C2 = (3,4), current point = (5,6), reflect: (5*2-3, 6*2-4) = (7, 8)
	ops, _ := parseSVGPathData("M 0 0 C 1 2 3 4 5 6 S 9 10 11 12")
	if ops[2].kind != 'C' {
		t.Fatalf("S not normalized to C, kind=%c", ops[2].kind)
	}
	if ops[2].args[0] != 7 || ops[2].args[1] != 8 {
		t.Errorf("S reflection wrong: c1=%g,%g, want 7,8", ops[2].args[0], ops[2].args[1])
	}
}

func TestParseSVGPathData_QuadBezier(t *testing.T) {
	ops, _ := parseSVGPathData("M 0 0 Q 1 2 3 4")
	if ops[1].kind != 'Q' {
		t.Fatalf("kind=%c", ops[1].kind)
	}
}

func TestParseSVGPathData_Close(t *testing.T) {
	ops, _ := parseSVGPathData("M 0 0 L 10 10 Z")
	if ops[2].kind != 'Z' {
		t.Errorf("Z not parsed, ops=%+v", ops)
	}
}

func TestParseSVGPathData_NoCommas(t *testing.T) {
	ops1, _ := parseSVGPathData("M0,0L10,10")
	ops2, _ := parseSVGPathData("M 0 0 L 10 10")
	if len(ops1) != len(ops2) || ops1[1].args[0] != ops2[1].args[0] {
		t.Errorf("comma vs space parsing differs")
	}
}

func TestParseSVGPathData_Arc(t *testing.T) {
	// Just verify it parses and endpoint is reached.
	ops, err := parseSVGPathData("M 0 0 A 5 5 0 1 0 10 0")
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) < 2 {
		t.Fatalf("expected M plus at least 1 C from arc decomposition, got %d ops", len(ops))
	}
	if ops[0].kind != 'M' || ops[1].kind != 'C' {
		t.Errorf("arc should decompose to C operators, got %c %c", ops[0].kind, ops[1].kind)
	}
	// End point of the decomposed arc must reach (10, 0)
	last := ops[len(ops)-1]
	if math.Abs(last.args[4]-10) > 1e-6 || math.Abs(last.args[5]) > 1e-6 {
		t.Errorf("arc endpoint = (%g, %g), want (10, 0)", last.args[4], last.args[5])
	}
}

// Regression: the cubic-Bezier approximation of a 90° circular arc must
// place its control points at the standard kappa distance k = (4/3)·tan(π/8)
// ≈ 0.5523·r from the endpoints. An older implementation used a different
// formula that under-estimated by ~32%, producing visibly pinched curves on
// any path with elliptical-arc commands (e.g. the Aspose logo's "O").
func TestDecomposeArc_90Degree_KappaControlDistance(t *testing.T) {
	// 90° arc from (10, 0) to (0, 10) on unit-scaled radius 10, no rotation,
	// large=0, sweep=1 (counter-clockwise short arc).
	ops := decomposeArcToBeziers(10, 0, 0, 10, 10, 10, 0, false, true)
	if len(ops) != 1 {
		t.Fatalf("90° arc should decompose to exactly 1 cubic, got %d", len(ops))
	}
	c := ops[0]
	if c.kind != 'C' {
		t.Fatalf("op kind = %c, want C", c.kind)
	}
	// Tangent at (10, 0) on a circle centered at (0, 0) going CCW is (0, 1).
	// Control point 1 should be at (10, 0) + k*(0, 1) = (10, k*10) for k ≈ 0.5523.
	expectedK := (4.0 / 3.0) * math.Tan(math.Pi/8) // ≈ 0.5522847
	c1x, c1y := c.args[0], c.args[1]
	if math.Abs(c1x-10) > 1e-6 {
		t.Errorf("control1 x = %g, want 10", c1x)
	}
	if math.Abs(c1y-10*expectedK) > 1e-4 {
		t.Errorf("control1 y = %g, want %g (kappa*r)", c1y, 10*expectedK)
	}
	// Tangent at (0, 10) is (-1, 0). Control point 2 should be at (0, 10) - k*(-1, 0) = (k*10, 10).
	c2x, c2y := c.args[2], c.args[3]
	if math.Abs(c2x-10*expectedK) > 1e-4 {
		t.Errorf("control2 x = %g, want %g (kappa*r)", c2x, 10*expectedK)
	}
	if math.Abs(c2y-10) > 1e-6 {
		t.Errorf("control2 y = %g, want 10", c2y)
	}
}

func TestParseSVGPathData_Malformed(t *testing.T) {
	_, err := parseSVGPathData("M 0")
	if err == nil {
		t.Error("expected error for incomplete M")
	}
}
