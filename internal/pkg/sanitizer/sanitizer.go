package sanitizer

import (
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

type Sanitizer struct {
	policy *bluemonday.Policy
}

func New() *Sanitizer {
	return &Sanitizer{
		policy: bluemonday.StrictPolicy(),
	}
}

func (s *Sanitizer) SanitizeHTML(input string) string {
	if input == "" {
		return ""
	}
	return s.policy.Sanitize(input)
}

func (s *Sanitizer) ValidateLength(input string, maxLength int) bool {
	return utf8.RuneCountInString(input) <= maxLength
}

func (s *Sanitizer) TrimWhitespace(input string) string {
	return strings.TrimSpace(input)
}

func (s *Sanitizer) ValidateMessageText(text string) (string, error) {
	if text == "" {
		return "", nil
	}

	text = s.TrimWhitespace(text)

	if !s.ValidateLength(text, 4096) {
		return "", ErrTextTooLong
	}

	text = s.SanitizeHTML(text)
	return text, nil
}

func (s *Sanitizer) ValidateChatName(name string) (string, error) {
	if name == "" {
		return "", ErrEmptyName
	}

	name = s.TrimWhitespace(name)

	if !s.ValidateLength(name, 100) {
		return "", ErrNameTooLong
	}

	name = s.SanitizeHTML(name)
	return name, nil
}
