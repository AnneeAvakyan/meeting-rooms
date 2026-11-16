package models

import (
	"testing"
	"time"
)

func TestBookingValidate(t *testing.T) {
	tests := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantErr bool
	}{
		{
			name:    "correct interval",
			start:   mustParseTime("2026-11-16T18:00:00+03:00"),
			end:     mustParseTime("2026-11-16T19:00:00+03:00"),
			wantErr: false,
		},
		{
			name:    "end before start",
			start:   mustParseTime("2026-11-16T20:00:00+03:00"),
			end:     mustParseTime("2026-11-16T19:00:00+03:00"),
			wantErr: true,
		},
		{
			name:    "start equals end",
			start:   mustParseTime("2026-11-16T19:00:00+03:00"),
			end:     mustParseTime("2026-11-16T19:00:00+03:00"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			booking := Booking{
				StartTime: tt.start,
				EndTime:   tt.end,
			}

			err := booking.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf(
					"Validate() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestBooking_Overlaps(t *testing.T) {
	tests := []struct {
		name        string
		roomIdA     int64
		roomIdB     int64
		startA      time.Time
		endA        time.Time
		startB      time.Time
		endB        time.Time
		wantOverlap bool
	}{
		{
			name:        "correct intervals",
			roomIdA:     1,
			roomIdB:     1,
			startA:      mustParseTime("2026-11-16T18:00:00+03:00"),
			endA:        mustParseTime("2026-11-16T19:00:00+03:00"),
			startB:      mustParseTime("2026-11-16T20:00:00+03:00"),
			endB:        mustParseTime("2026-11-16T21:00:00+03:00"),
			wantOverlap: false,
		},
		{
			name:        "correct intervals different rooms",
			roomIdA:     1,
			roomIdB:     2,
			startA:      mustParseTime("2026-11-16T18:00:00+03:00"),
			endA:        mustParseTime("2026-11-16T19:00:00+03:00"),
			startB:      mustParseTime("2026-11-16T20:00:00+03:00"),
			endB:        mustParseTime("2026-11-16T21:00:00+03:00"),
			wantOverlap: false,
		},
		{
			name:        "overlaps intervals",
			roomIdA:     1,
			roomIdB:     1,
			startA:      mustParseTime("2026-11-16T18:00:00+03:00"),
			endA:        mustParseTime("2026-11-16T19:00:00+03:00"),
			startB:      mustParseTime("2026-11-16T18:30:00+03:00"),
			endB:        mustParseTime("2026-11-16T21:00:00+03:00"),
			wantOverlap: true,
		},
		{
			name:        "overlaps intervals but different rooms",
			roomIdA:     1,
			roomIdB:     2,
			startA:      mustParseTime("2026-11-16T18:00:00+03:00"),
			endA:        mustParseTime("2026-11-16T19:00:00+03:00"),
			startB:      mustParseTime("2026-11-16T18:30:00+03:00"),
			endB:        mustParseTime("2026-11-16T21:00:00+03:00"),
			wantOverlap: false,
		},
		{
			name:        "overlaps intervals end and new start",
			roomIdA:     1,
			roomIdB:     1,
			startA:      mustParseTime("2026-11-16T18:00:00+03:00"),
			endA:        mustParseTime("2026-11-16T19:00:00+03:00"),
			startB:      mustParseTime("2026-11-16T19:00:00+03:00"),
			endB:        mustParseTime("2026-11-16T21:00:00+03:00"),
			wantOverlap: false,
		},
		{
			name:        "correct intervals",
			roomIdA:     1,
			roomIdB:     1,
			startA:      mustParseTime("2026-11-16T18:00:00+03:00"),
			endA:        mustParseTime("2026-11-16T19:00:00+03:00"),
			startB:      mustParseTime("2026-11-16T19:00:01+03:00"),
			endB:        mustParseTime("2026-11-16T21:00:00+03:00"),
			wantOverlap: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bookingA := Booking{
				RoomID:    tt.roomIdA,
				StartTime: tt.startA,
				EndTime:   tt.endA,
			}
			bookingB := Booking{
				RoomID:    tt.roomIdB,
				StartTime: tt.startB,
				EndTime:   tt.endB,
			}

			res := bookingA.Overlaps(bookingB)

			if res != tt.wantOverlap {
				t.Errorf("Overlaps() = %v, want %v", res, tt.wantOverlap)
			}
		})
	}
}

func TestHasOverlap(t *testing.T) {
	tests := []struct {
		name        string
		candidate   Booking
		existing    []Booking
		wantOverlap bool
	}{
		{
			name: "no overlaps, 1 room, all active",
			candidate: Booking{
				RoomID:    1,
				StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
				EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
				Status:    StatusActive,
			},
			existing: []Booking{
				{
					RoomID:    1,
					StartTime: mustParseTime("2026-11-16T16:00:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T17:00:00+03:00"),
					Status:    StatusActive,
				},
				{
					RoomID:    1,
					StartTime: mustParseTime("2026-11-16T17:00:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T18:00:00+03:00"),
					Status:    StatusActive,
				},
			},
			wantOverlap: false,
		},
		{
			name: "overlaps, 1 room, all active",
			candidate: Booking{
				RoomID:    1,
				StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
				EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
				Status:    StatusActive,
			},
			existing: []Booking{
				{
					RoomID:    1,
					StartTime: mustParseTime("2026-11-16T18:30:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T20:00:00+03:00"),
					Status:    StatusActive,
				},
				{
					RoomID:    1,
					StartTime: mustParseTime("2026-11-17T18:30:00+03:00"),
					EndTime:   mustParseTime("2026-11-17T20:00:00+03:00"),
					Status:    StatusActive,
				},
			},
			wantOverlap: true,
		},
		{
			name: "overlaps with canceled, 1 room (no actual overlap)",
			candidate: Booking{
				RoomID:    1,
				StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
				EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
				Status:    StatusActive,
			},
			existing: []Booking{
				{
					RoomID:    1,
					StartTime: mustParseTime("2026-11-16T18:30:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T20:00:00+03:00"),
					Status:    StatusCancelled,
				},
				{
					RoomID:    1,
					StartTime: mustParseTime("2026-11-17T18:30:00+03:00"),
					EndTime:   mustParseTime("2026-11-17T20:00:00+03:00"),
					Status:    StatusActive,
				},
			},
			wantOverlap: false,
		},
		{
			name: "overlaps with different room (no actual overlap)",
			candidate: Booking{
				RoomID:    1,
				StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
				EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
				Status:    StatusActive,
			},
			existing: []Booking{
				{
					RoomID:    2,
					StartTime: mustParseTime("2026-11-16T18:30:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T20:00:00+03:00"),
					Status:    StatusActive,
				},
				{
					RoomID:    1,
					StartTime: mustParseTime("2026-11-17T18:30:00+03:00"),
					EndTime:   mustParseTime("2026-11-17T20:00:00+03:00"),
					Status:    StatusActive,
				},
			},
			wantOverlap: false,
		},
		{
			name: "update booking without self overlap",
			candidate: Booking{
				ID:        5,
				RoomID:    1,
				StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
				EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
				Status:    StatusActive,
			},
			existing: []Booking{
				{
					ID:        5,
					RoomID:    1,
					StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
					Status:    StatusActive,
				},
			},
			wantOverlap: false,
		},
		{
			name: "empty existing bookings",
			candidate: Booking{
				RoomID:    1,
				StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
				EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
				Status:    StatusActive,
			},
			existing:    []Booking{},
			wantOverlap: false,
		},
		{
			name: "conflict after updating booking",
			candidate: Booking{
				ID:        5,
				RoomID:    1,
				StartTime: mustParseTime("2026-11-16T18:30:00+03:00"),
				EndTime:   mustParseTime("2026-11-16T19:30:00+03:00"),
				Status:    StatusActive,
			},
			existing: []Booking{
				{
					ID:        5,
					RoomID:    1,
					StartTime: mustParseTime("2026-11-16T19:00:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T20:00:00+03:00"),
					Status:    StatusActive,
				},
				{
					ID:        6,
					RoomID:    1,
					StartTime: mustParseTime("2026-11-16T18:00:00+03:00"),
					EndTime:   mustParseTime("2026-11-16T19:00:00+03:00"),
					Status:    StatusActive,
				},
			},
			wantOverlap: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := tt.candidate
			existing := tt.existing

			res := HasOverlap(candidate, existing)

			if res != tt.wantOverlap {
				t.Errorf("HasOverlap() = %v, want %v", res, tt.wantOverlap)
			}
		})
	}
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
