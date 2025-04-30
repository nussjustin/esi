package text

import (
	"errors"
	"fmt"
)

// UnexpectedCharacterError is returned by [Scanner.ConsumeOrError] when the next character does not match the expected.
type UnexpectedCharacterError struct {
	// At is the position at which the error occurred.
	At int

	// Got is the character that was read.
	Got byte

	// Expected contains the expected character.
	Expected byte
}

// Error returns a human-readable error message.
func (u *UnexpectedCharacterError) Error() string {
	return fmt.Sprintf("unexpected character '%c' at offset %d, '%c' expected", u.Got, u.At, u.Expected)
}

// Is returns true if the given error is the same as the receiver.
func (u *UnexpectedCharacterError) Is(err error) bool {
	var o *UnexpectedCharacterError
	return errors.As(err, &o) && *o == *u
}

// Offset returns u.At.
func (u *UnexpectedCharacterError) Offset() int {
	return u.At
}

// UnexpectedEndOfInput is returned by [Scanner.ConsumeOrError] when there is no character to read.
type UnexpectedEndOfInput struct {
	// At is the position at which the error occurred.
	At int

	// Expected contains the expected character.
	Expected byte
}

// Error returns a human-readable error message.
func (u *UnexpectedEndOfInput) Error() string {
	return fmt.Sprintf("unexpected end of input at offset %d, character %c expected", u.At, u.Expected)
}

// Is returns true if the given error is the same as the receiver.
func (u *UnexpectedEndOfInput) Is(err error) bool {
	var o *UnexpectedEndOfInput
	return errors.As(err, &o) && *o == *u
}

// Offset returns u.At.
func (u *UnexpectedEndOfInput) Offset() int {
	return u.At
}

// Scanner implements methods for scanning bytes from strings and []byte and is meant to be used in building
// higher-level scanners on top of it.
type Scanner[T []byte | string] struct {
	data   T
	offset int
}

// Consume checks if the next byte is equal to c and, if true, advances the scanner.
func (s *Scanner[T]) Consume(c byte) bool {
	c1, ok := s.Peek()
	if !ok || c != c1 {
		return false
	}
	s.offset++
	return true
}

// ConsumeOrError is like Consume, but returns an error if the next character is not c or there is no more input.
func (s *Scanner[T]) ConsumeOrError(c byte) error {
	c1, ok := s.Peek()
	if !ok {
		return &UnexpectedEndOfInput{At: s.offset, Expected: c}
	}
	if c1 != c {
		return &UnexpectedCharacterError{At: s.offset, Got: c1, Expected: c}
	}
	s.offset++
	return nil
}

// Offset returns the current offset in the data.
func (s *Scanner[T]) Offset() int {
	return s.offset
}

// Peek returns the next byte in the input.
func (s *Scanner[T]) Peek() (byte, bool) {
	if s.offset >= len(s.data) {
		return 0, false
	}
	return s.data[s.offset], true
}

// Reset resets the internal state of the scanner and sets it to read from data.
func (s *Scanner[T]) Reset(data T) {
	s.data = data
	s.offset = 0
}

// SkipWhile skips over bytes in the input until f returns false.
func (s *Scanner[T]) SkipWhile(f func(c byte) bool) bool {
	for s.offset < len(s.data) && f(s.data[s.offset]) {
		s.offset++
	}

	return s.offset < len(s.data)
}

// Skip skips over ASCII spaces inside the input.
func (s *Scanner[T]) SkipSpaces() {
	for s.offset < len(s.data) {
		switch s.data[s.offset] {
		case ' ', '\r', '\n', '\t':
			s.offset++
		default:
			return
		}
	}
}

// Unread undoes the last byte read, moving the offset back one byte.
//
// If there is no previous byte, Unread panics.
func (s *Scanner[T]) Unread() {
	if s.offset <= 0 {
		panic("nothing to unread")
	}

	s.offset--
}
