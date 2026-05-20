package asposepdf

import "testing"

func TestPath_NewIsEmpty(t *testing.T) {
	p := NewPath()
	if p == nil {
		t.Fatal("NewPath returned nil")
	}
	if len(p.ops) != 0 {
		t.Errorf("ops = %d, want 0", len(p.ops))
	}
}

func TestPath_MoveToLineToClose(t *testing.T) {
	p := NewPath().MoveTo(10, 20).LineTo(30, 40).Close()
	if len(p.ops) != 3 {
		t.Fatalf("ops = %d, want 3", len(p.ops))
	}
	if p.ops[0].kind != pathOpMoveTo || p.ops[0].x != 10 || p.ops[0].y != 20 {
		t.Errorf("op[0] = %+v", p.ops[0])
	}
	if p.ops[1].kind != pathOpLineTo || p.ops[1].x != 30 || p.ops[1].y != 40 {
		t.Errorf("op[1] = %+v", p.ops[1])
	}
	if p.ops[2].kind != pathOpClose {
		t.Errorf("op[2].kind = %v, want pathOpClose", p.ops[2].kind)
	}
}

func TestPath_Chaining(t *testing.T) {
	p := NewPath().MoveTo(0, 0).LineTo(1, 1).LineTo(2, 0).LineTo(1, -1).Close()
	if len(p.ops) != 5 {
		t.Errorf("len = %d, want 5", len(p.ops))
	}
}
