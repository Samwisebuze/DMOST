package session

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestJoinAndEnd(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Code()) < 12 {
		t.Fatal("join code too short")
	}
	if _, _, err := s.Join("wrong", "Ada"); !errors.Is(err, ErrCode) {
		t.Fatalf("wrong code: %v", err)
	}
	token, p, err := s.Join(s.Code(), "Ada")
	if err != nil || token == "" || p.Name != "Ada" {
		t.Fatalf("join: %q %+v %v", token, p, err)
	}
	if _, err := s.Authenticate(token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(token, "hello"); err != nil {
		t.Fatal(err)
	}
	s.End()
	if _, err := s.Send(token, "again"); !errors.Is(err, ErrEnded) {
		t.Fatalf("send after end: %v", err)
	}
	if _, _, err := s.Join(s.Code(), "Bob"); !errors.Is(err, ErrEnded) {
		t.Fatalf("join after end: %v", err)
	}
}

func TestFanoutReplayAndPresence(t *testing.T) {
	s, _ := New()
	a, _, _ := s.Join(s.Code(), "Ada")
	b, _, _ := s.Join(s.Code(), "Bob")
	_, ach, acancel, err := s.Subscribe(a, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer acancel()
	_, bch, bcancel, err := s.Subscribe(b, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer bcancel()
	m1, err := s.Send(a, "one")
	if err != nil {
		t.Fatal(err)
	}
	m2, err := s.Send(b, "two")
	if err != nil {
		t.Fatal(err)
	}
	if m2.ID <= m1.ID {
		t.Fatal("messages out of order")
	}
	for _, ch := range []<-chan Event{ach, bch} {
		found := 0
		deadline := time.After(time.Second)
		for found < 2 {
			select {
			case e := <-ch:
				if e.Type == "message" {
					found++
				}
			case <-deadline:
				t.Fatalf("subscriber saw %d messages", found)
			}
		}
	}
	replay, _, cancel, err := s.Subscribe(a, m1.ID)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if len(replay) == 0 || replay[len(replay)-1].Message.Text != "two" {
		t.Fatalf("replay: %+v", replay)
	}
	ps, _, err := s.Snapshot(a)
	if err != nil || len(ps) != 2 {
		t.Fatalf("snapshot: %+v %v", ps, err)
	}
}

func TestLimitsAndExpiredReplay(t *testing.T) {
	s, _ := New()
	token, _, _ := s.Join(s.Code(), "Ada")
	if _, err := s.Send(token, strings.Repeat("x", maxMessageBytes+1)); !errors.Is(err, ErrMessage) {
		t.Fatal(err)
	}
	for i := 0; i < maxHistory+5; i++ {
		if _, err := s.Send(token, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err := s.Subscribe(token, 1); !errors.Is(err, ErrReplayExpired) {
		t.Fatalf("replay: %v", err)
	}
	for i := 1; i < maxParticipants; i++ {
		if _, _, err := s.Join(s.Code(), "guest"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := s.Join(s.Code(), "extra"); !errors.Is(err, ErrFull) {
		t.Fatalf("limit: %v", err)
	}
}
