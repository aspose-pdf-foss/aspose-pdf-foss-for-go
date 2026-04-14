package asposepdf

import (
	"strings"
	"testing"
)

func TestSerializeContentOps(t *testing.T) {
	// Build a simple content stream, parse it, serialize it, parse again.
	original := "q\n10 0 0 20 50.5 100 cm\n/Im0 Do\nQ\n"
	ops, err := parseContentStream([]byte(original))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	serialized := serializeContentOps(ops)
	result := string(serialized)

	// Re-parse and verify structural equivalence.
	ops2, err := parseContentStream(serialized)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if len(ops2) != len(ops) {
		t.Fatalf("op count: got %d, want %d", len(ops2), len(ops))
	}

	for i, op := range ops {
		if ops2[i].Operator != op.Operator {
			t.Errorf("op[%d]: got %q, want %q", i, ops2[i].Operator, op.Operator)
		}
		if len(ops2[i].Operands) != len(op.Operands) {
			t.Errorf("op[%d] operands: got %d, want %d", i, len(ops2[i].Operands), len(op.Operands))
		}
	}

	// Verify key operators are present in output.
	if !strings.Contains(result, "cm") {
		t.Error("serialized should contain cm")
	}
	if !strings.Contains(result, "Do") {
		t.Error("serialized should contain Do")
	}
	if !strings.Contains(result, "/Im0") {
		t.Error("serialized should contain /Im0")
	}
}

func TestSerializeContentOpsWithText(t *testing.T) {
	// Content stream with text operators and TJ array.
	original := "BT\n/F1 12 Tf\n100 700 Td\n[(Hello) -50 (World)] TJ\nET\n"
	ops, err := parseContentStream([]byte(original))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	serialized := serializeContentOps(ops)
	ops2, err := parseContentStream(serialized)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if len(ops2) != len(ops) {
		t.Fatalf("op count: got %d, want %d", len(ops2), len(ops))
	}

	// Verify TJ operator preserved.
	found := false
	for _, op := range ops2 {
		if op.Operator == "TJ" {
			found = true
			if len(op.Operands) != 1 {
				t.Errorf("TJ operands: got %d, want 1", len(op.Operands))
			}
		}
	}
	if !found {
		t.Error("TJ operator not found in re-parsed output")
	}
}
