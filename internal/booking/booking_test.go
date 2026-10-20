package booking

import (
	"testing"
	"time"
)

func mkBooking(id, roomID int64, startOffset, endOffset time.Duration, status Status) Booking {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	return Booking{
		ID:        id,
		RoomID:    roomID,
		StartTime: base.Add(startOffset),
		EndTime:   base.Add(endOffset),
		Status:    status,
	}
}

func TestOverlaps(t *testing.T) {
	h := time.Hour

	cases := []struct {
		name     string
		a, b     Booking
		expected bool
	}{
		{
			name:     "identical intervals overlap",
			a:        mkBooking(1, 1, 0, h, StatusActive),
			b:        mkBooking(2, 1, 0, h, StatusActive),
			expected: true,
		},
		{
			name:     "partial overlap",
			a:        mkBooking(1, 1, 0, 2*h, StatusActive),
			b:        mkBooking(2, 1, h, 3*h, StatusActive),
			expected: true,
		},
		{
			name:     "nested interval overlaps",
			a:        mkBooking(1, 1, 0, 3*h, StatusActive),
			b:        mkBooking(2, 1, h, 2*h, StatusActive),
			expected: true,
		},
		{
			name:     "back-to-back does not overlap (end == start)",
			a:        mkBooking(1, 1, 0, h, StatusActive),
			b:        mkBooking(2, 1, h, 2*h, StatusActive),
			expected: false,
		},
		{
			name:     "different rooms never overlap",
			a:        mkBooking(1, 1, 0, h, StatusActive),
			b:        mkBooking(2, 2, 0, h, StatusActive),
			expected: false,
		},
		{
			name:     "fully separate intervals",
			a:        mkBooking(1, 1, 0, h, StatusActive),
			b:        mkBooking(2, 1, 2*h, 3*h, StatusActive),
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.a.Overlaps(c.b)
			if got != c.expected {
				t.Errorf("Overlaps() = %v, want %v", got, c.expected)
			}
			// Проверка пересечения должна быть симметричной.
			if gotRev := c.b.Overlaps(c.a); gotRev != got {
				t.Errorf("Overlaps() not symmetric: a.Overlaps(b)=%v, b.Overlaps(a)=%v", got, gotRev)
			}
		})
	}
}

func TestHasOverlap(t *testing.T) {
	h := time.Hour
	existing := []Booking{
		mkBooking(1, 1, 0, h, StatusActive),
		mkBooking(2, 1, 3*h, 4*h, StatusCancelled), // отменённая — не должна мешать
	}

	t.Run("conflicts with active booking", func(t *testing.T) {
		candidate := mkBooking(0, 1, 30*time.Minute, 90*time.Minute, StatusActive)
		if !HasOverlap(candidate, existing) {
			t.Error("expected overlap with active booking")
		}
	})

	t.Run("no conflict with cancelled booking", func(t *testing.T) {
		candidate := mkBooking(0, 1, 3*h, 4*h, StatusActive)
		if HasOverlap(candidate, existing) {
			t.Error("cancelled booking should not count as conflict")
		}
	})

	t.Run("editing self is not a conflict", func(t *testing.T) {
		candidate := mkBooking(1, 1, 0, h, StatusActive) // то же ID, что и existing[0]
		if HasOverlap(candidate, existing) {
			t.Error("booking should not conflict with itself during edit")
		}
	})
}

func TestValidate(t *testing.T) {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	valid := Booking{StartTime: base, EndTime: base.Add(time.Hour)}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid booking, got error: %v", err)
	}

	invalid := Booking{StartTime: base, EndTime: base}
	if err := invalid.Validate(); err != ErrInvalidInterval {
		t.Errorf("expected ErrInvalidInterval, got: %v", err)
	}
}
