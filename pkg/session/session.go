// Package session owns the ephemeral state of one local table.
package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
)

var (
	ErrCode          = errors.New("invalid join code")
	ErrEnded         = errors.New("session ended")
	ErrToken         = errors.New("invalid participant token")
	ErrName          = errors.New("invalid participant name")
	ErrMessage       = errors.New("invalid message")
	ErrFull          = errors.New("session full")
	ErrReplayExpired = errors.New("replay window expired")
)

const (
	maxParticipants = 32
	maxNameBytes    = 60
	maxMessageBytes = 2000
	maxHistory      = 256
)

type Participant struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Online bool   `json:"online"`
}

type Message struct {
	ID          uint64 `json:"id"`
	Participant string `json:"participant"`
	Text        string `json:"text"`
}

type Event struct {
	Sequence     uint64        `json:"sequence"`
	Type         string        `json:"type"`
	Participant  *Participant  `json:"participant,omitempty"`
	Message      *Message      `json:"message,omitempty"`
	Participants []Participant `json:"participants,omitempty"`
}

type subscription struct {
	participant string
	ch          chan Event
}

// Session is safe for concurrent HTTP handlers. All state is transient.
type Session struct {
	mu           sync.Mutex
	code         string
	ended        bool
	sequence     uint64
	participants map[string]Participant // token to participant
	history      []Event
	subscribers  map[uint64]subscription
	nextSub      uint64
}

func New() (*Session, error) {
	raw := make([]byte, 9)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return &Session{
		code:         strings.ToUpper(base64.RawURLEncoding.EncodeToString(raw)),
		participants: make(map[string]Participant),
		subscribers:  make(map[uint64]subscription),
	}, nil
}

func (s *Session) Code() string { return s.code }

func (s *Session) Join(code, name string) (string, Participant, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxNameBytes {
		return "", Participant{}, ErrName
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToUpper(code)), []byte(s.code)) != 1 {
		return "", Participant{}, ErrCode
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", Participant{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	p := Participant{ID: token[:12], Name: name}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return "", Participant{}, ErrEnded
	}
	if len(s.participants) >= maxParticipants {
		return "", Participant{}, ErrFull
	}
	s.participants[token] = p
	s.publishLocked(Event{Type: "participant", Participant: &p})
	return token, p, nil
}

func (s *Session) Authenticate(token string) (Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.participants[token]
	if !ok {
		return Participant{}, ErrToken
	}
	if s.ended {
		return Participant{}, ErrEnded
	}
	return p, nil
}

func (s *Session) Send(token, text string) (Message, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxMessageBytes {
		return Message{}, ErrMessage
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return Message{}, ErrEnded
	}
	p, ok := s.participants[token]
	if !ok {
		return Message{}, ErrToken
	}
	message := Message{ID: s.sequence + 1, Participant: p.ID, Text: text}
	s.publishLocked(Event{Type: "message", Message: &message})
	return message, nil
}

// Subscribe returns a replay from after and a live stream. Cancel must be called.
// A slow subscriber is dropped; reconnect with the last received event ID.
func (s *Session) Subscribe(token string, after uint64) ([]Event, <-chan Event, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return nil, nil, nil, ErrEnded
	}
	p, ok := s.participants[token]
	if !ok {
		return nil, nil, nil, ErrToken
	}
	if len(s.history) > 0 && after != 0 && after+1 < s.history[0].Sequence {
		return nil, nil, nil, ErrReplayExpired
	}
	replay := make([]Event, 0)
	for _, e := range s.history {
		if e.Sequence > after {
			replay = append(replay, e)
		}
	}
	s.nextSub++
	id := s.nextSub
	ch := make(chan Event, 32)
	s.subscribers[id] = subscription{participant: token, ch: ch}
	if !p.Online {
		p.Online = true
		s.participants[token] = p
		s.publishLocked(Event{Type: "participant", Participant: &p})
	}
	cancel := func() { s.unsubscribe(id) }
	return replay, ch, cancel, nil
}

func (s *Session) Snapshot(token string) ([]Participant, uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return nil, 0, ErrEnded
	}
	if _, ok := s.participants[token]; !ok {
		return nil, 0, ErrToken
	}
	ps := make([]Participant, 0, len(s.participants))
	for _, p := range s.participants {
		ps = append(ps, p)
	}
	return ps, s.sequence, nil
}

func (s *Session) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.publishLocked(Event{Type: "ended"})
	s.ended = true
	for id, sub := range s.subscribers {
		close(sub.ch)
		delete(s.subscribers, id)
	}
}

func (s *Session) unsubscribe(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscribers[id]
	if !ok {
		return
	}
	delete(s.subscribers, id)
	close(sub.ch)
	for _, other := range s.subscribers {
		if other.participant == sub.participant {
			return
		}
	}
	p := s.participants[sub.participant]
	p.Online = false
	s.participants[sub.participant] = p
	if !s.ended {
		s.publishLocked(Event{Type: "participant", Participant: &p})
	}
}

func (s *Session) publishLocked(e Event) {
	s.sequence++
	e.Sequence = s.sequence
	s.history = append(s.history, e)
	if len(s.history) > maxHistory {
		s.history = s.history[len(s.history)-maxHistory:]
	}
	for id, sub := range s.subscribers {
		select {
		case sub.ch <- e:
		default:
			close(sub.ch)
			delete(s.subscribers, id)
		}
	}
}
