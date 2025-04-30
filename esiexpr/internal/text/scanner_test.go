package text_test

import (
	"testing"

	"github.com/nussjustin/esi/esiexpr/internal/text"
)

func TestScanner_Consume(t *testing.T) {
	var s text.Scanner[string]
	s.Reset("hey")

	if s.Consume('H') {
		t.Error("Consume('H') = true, want false")
	}

	if s.Offset() != 0 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 0)
	}

	if !s.Consume('h') {
		t.Error("Consume('h') = false, want true")
	}

	if s.Offset() != 1 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 1)
	}

	_ = s.Consume('e')
	_ = s.Consume('y')

	if s.Offset() != 3 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 3)
	}

	if s.Consume(0) {
		t.Error("Consume(0) = true, want false")
	}
}

func TestScanner_ConsumeOrError(t *testing.T) {
	var s text.Scanner[string]
	s.Reset("hey")

	if err := s.ConsumeOrError('H'); err == nil {
		t.Error("ConsumeOrError('H') = <nil>, want non-nil error")
	}

	if s.Offset() != 0 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 0)
	}

	if err := s.ConsumeOrError('h'); err != nil {
		t.Errorf("ConsumeOrError('h') = %v, want <nil>", err)
	}

	if s.Offset() != 1 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 1)
	}

	_ = s.ConsumeOrError('e')
	_ = s.ConsumeOrError('y')

	if s.Offset() != 3 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 3)
	}

	if err := s.ConsumeOrError(0); err == nil {
		t.Error("ConsumeOrError(0) = <nil>, want non-nil error")
	}
}

func TestScanner_Peek(t *testing.T) {
	var s text.Scanner[string]
	s.Reset("hey")

	if c, ok := s.Peek(); c != 'h' || !ok {
		t.Errorf("Peek() = (%c, %t), want (h, true)", c, ok)
	}

	if c, ok := s.Peek(); c != 'h' || !ok {
		t.Errorf("Peek() = (%c, %t), want (h, true)", c, ok)
	}

	if s.Offset() != 0 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 0)
	}

	_ = s.Consume('h')

	if c, ok := s.Peek(); c != 'e' || !ok {
		t.Errorf("Peek() = (%c, %t), want (e, true)", c, ok)
	}

	if c, ok := s.Peek(); c != 'e' || !ok {
		t.Errorf("Peek() = (%c, %t), want (e, true)", c, ok)
	}

	if s.Offset() != 1 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 1)
	}

	_ = s.Consume('e')
	_ = s.Consume('y')

	if c, ok := s.Peek(); c != 0 || ok {
		t.Errorf("Peek() = (%c, %t), want (0, false)", c, ok)
	}

	if c, ok := s.Peek(); c != 0 || ok {
		t.Errorf("Peek() = (%c, %t), want (0, false)", c, ok)
	}

	if s.Offset() != 3 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 3)
	}
}

func TestScanner_Reset(t *testing.T) {
	var s text.Scanner[string]
	s.Reset("hey")

	_ = s.Consume('h')

	if c, ok := s.Peek(); c != 'e' || !ok {
		t.Errorf("Peek() = (%c, %t), want (e, true)", c, ok)
	}

	if s.Offset() != 1 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 1)
	}

	s.Reset("foo")

	if c, ok := s.Peek(); c != 'f' || !ok {
		t.Errorf("Peek() = (%c, %t), want (e, true)", c, ok)
	}

	if s.Offset() != 0 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 0)
	}
}

func TestScanner_SkipSpaces(t *testing.T) {
	var s text.Scanner[string]
	s.Reset(" \t\r\nh    ey")

	s.SkipSpaces()

	if s.Offset() != 4 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 4)
	}

	if !s.Consume('h') {
		t.Error("Consume('h') = false, want true")
	}

	s.SkipSpaces()

	if s.Offset() != 9 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 9)
	}
}

func TestScanner_SkipWhile(t *testing.T) {
	var s text.Scanner[string]
	s.Reset("--hey")

	s.SkipWhile(func(c byte) bool {
		return c == '-'
	})

	if s.Offset() != 2 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 2)
	}

	s.SkipWhile(func(c byte) bool {
		return c == '-'
	})

	if s.Offset() != 2 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 2)
	}

	s.SkipWhile(func(c byte) bool {
		return c == 'h'
	})

	if s.Offset() != 3 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 2)
	}
}

func TestScanner_Unread(t *testing.T) {
	var s text.Scanner[string]
	s.Reset("hey")

	_ = s.Consume('h')
	_ = s.Consume('e')

	if c, ok := s.Peek(); c != 'y' || !ok {
		t.Errorf("Peek() = (%c, %t), want (y, true)", c, ok)
	}

	if s.Offset() != 2 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 2)
	}

	s.Unread()

	if c, ok := s.Peek(); c != 'e' || !ok {
		t.Errorf("Peek() = (%c, %t), want (e, true)", c, ok)
	}

	if s.Offset() != 1 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 1)
	}

	s.Unread()

	if c, ok := s.Peek(); c != 'h' || !ok {
		t.Errorf("Peek() = (%c, %t), want (h, true)", c, ok)
	}

	if s.Offset() != 0 {
		t.Errorf("Offset() = %d, want %d", s.Offset(), 0)
	}
}
