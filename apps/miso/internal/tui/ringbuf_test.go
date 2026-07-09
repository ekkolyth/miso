package tui

import (
	"testing"
)

func TestRingBuffer_Basic(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Write("line1")
	rb.Write("line2")
	rb.Write("line3")

	lines := rb.Lines()
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	expectedLines := []string{"line1", "line2", "line3"}
	for i, expected := range expectedLines {
		if i >= len(lines) || lines[i] != expected {
			t.Errorf("line %d: expected %q, got %q", i, expected, lines[i])
		}
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Write("line1")
	rb.Write("line2")
	rb.Write("line3")
	rb.Write("line4") // should drop line1

	lines := rb.Lines()
	if len(lines) != 3 {
		t.Errorf("expected 3 lines after overflow, got %d", len(lines))
	}

	expectedLines := []string{"line2", "line3", "line4"}
	for i, expected := range expectedLines {
		if i >= len(lines) || lines[i] != expected {
			t.Errorf("line %d: expected %q, got %q", i, expected, lines[i])
		}
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Write("line1")
	rb.Write("line2")
	rb.Write("line3")

	rb.Clear()

	lines := rb.Lines()
	if len(lines) != 0 {
		t.Errorf("expected 0 lines after clear, got %d", len(lines))
	}

	if rb.Len() != 0 {
		t.Errorf("expected Len() to return 0 after clear, got %d", rb.Len())
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(3)

	if rb.Len() != 0 {
		t.Errorf("expected new buffer to have 0 lines, got %d", rb.Len())
	}

	lines := rb.Lines()
	if len(lines) != 0 {
		t.Errorf("expected new buffer to return empty slice, got %d lines", len(lines))
	}
}

func TestRingBufferBaseSeq(t *testing.T) {
	rb := NewRingBuffer(3)
	if rb.BaseSeq() != 0 {
		t.Fatalf("empty BaseSeq = %d, want 0", rb.BaseSeq())
	}
	rb.Write("a")
	rb.Write("b")
	rb.Write("c")
	if rb.BaseSeq() != 0 {
		t.Errorf("BaseSeq before eviction = %d, want 0", rb.BaseSeq())
	}
	rb.Write("d") // evicts "a"
	if rb.BaseSeq() != 1 {
		t.Errorf("BaseSeq after 1 eviction = %d, want 1", rb.BaseSeq())
	}
	if lines := rb.Lines(); lines[0] != "b" {
		t.Errorf("oldest retained = %q, want %q", lines[0], "b")
	}
}
