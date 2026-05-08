package enno

type Session struct {
	Messages []Message

	lastCompleteInputTokens int64
}

func (s *Session) Append(message Message) {
	if s == nil {
		return
	}
	s.Messages = append(s.Messages, cloneMessage(message))
}

func (s Session) Clone() Session {
	return Session{
		Messages:                cloneMessages(s.Messages),
		lastCompleteInputTokens: s.lastCompleteInputTokens,
	}
}

func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.Messages = nil
	s.lastCompleteInputTokens = 0
}
