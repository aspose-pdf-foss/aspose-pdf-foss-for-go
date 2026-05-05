package asposepdf

import "testing"

func TestBuilderPushPopState(t *testing.T) {
	b := newAppearanceBuilder()
	b.PushState()
	b.PopState()
	if got := string(b.Bytes()); got != "q\nQ\n" {
		t.Errorf("got %q, want \"q\\nQ\\n\"", got)
	}
}

func TestBuilderConcatMatrix(t *testing.T) {
	b := newAppearanceBuilder()
	b.ConcatMatrix(1, 0, 0, 1, 10, 20)
	if got := string(b.Bytes()); got != "1 0 0 1 10 20 cm\n" {
		t.Errorf("got %q", got)
	}
}

func TestBuilderSetLineWidth(t *testing.T) {
	b := newAppearanceBuilder()
	b.SetLineWidth(2.5)
	if got := string(b.Bytes()); got != "2.5 w\n" {
		t.Errorf("got %q", got)
	}
}

func TestBuilderSetLineCap(t *testing.T) {
	for _, tc := range []struct {
		cap  LineCap
		want string
	}{
		{LineCapButt, "0 J\n"},
		{LineCapRound, "1 J\n"},
		{LineCapSquare, "2 J\n"},
	} {
		b := newAppearanceBuilder()
		b.SetLineCap(tc.cap)
		if got := string(b.Bytes()); got != tc.want {
			t.Errorf("cap=%v: got %q, want %q", tc.cap, got, tc.want)
		}
	}
}

func TestBuilderSetLineJoin(t *testing.T) {
	for _, tc := range []struct {
		join LineJoin
		want string
	}{
		{LineJoinMiter, "0 j\n"},
		{LineJoinRound, "1 j\n"},
		{LineJoinBevel, "2 j\n"},
	} {
		b := newAppearanceBuilder()
		b.SetLineJoin(tc.join)
		if got := string(b.Bytes()); got != tc.want {
			t.Errorf("join=%v: got %q, want %q", tc.join, got, tc.want)
		}
	}
}

func TestBuilderSetMiterLimit(t *testing.T) {
	b := newAppearanceBuilder()
	b.SetMiterLimit(10)
	if got := string(b.Bytes()); got != "10 M\n" {
		t.Errorf("got %q", got)
	}
}

func TestBuilderSetDashPattern(t *testing.T) {
	b := newAppearanceBuilder()
	b.SetDashPattern([]float64{3, 3}, 0)
	if got := string(b.Bytes()); got != "[3 3] 0 d\n" {
		t.Errorf("got %q", got)
	}
}

func TestBuilderSetDashPatternEmpty(t *testing.T) {
	b := newAppearanceBuilder()
	b.SetDashPattern(nil, 0)
	if got := string(b.Bytes()); got != "[] 0 d\n" {
		t.Errorf("got %q", got)
	}
}
