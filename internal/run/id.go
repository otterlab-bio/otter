package run

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

const randomSuffixLength = 6

func GenerateID() (string, error) {
	return generateID(time.Now().UTC(), rand.Reader)
}

func ValidateID(runID string) error {
	if !configv1.IsValidRunID(runID) {
		return fmt.Errorf("run ID %q must match run-YYYYMMDDTHHMMSSZ-abcdef", runID)
	}
	return nil
}

func CreatedAtFromID(runID string) (time.Time, error) {
	if err := ValidateID(runID); err != nil {
		return time.Time{}, err
	}
	createdAt, err := time.Parse("20060102T150405Z", runID[4:20])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse run ID timestamp: %w", err)
	}
	return createdAt.UTC(), nil
}

func generateID(createdAt time.Time, randomSource io.Reader) (string, error) {
	suffix, err := randomLetters(randomSource, randomSuffixLength)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("run-%s-%s", createdAt.UTC().Format("20060102T150405Z"), suffix), nil
}

func randomLetters(randomSource io.Reader, length int) (string, error) {
	encoded := make([]byte, 0, length)
	buffer := []byte{0}
	for len(encoded) < length {
		if _, err := io.ReadFull(randomSource, buffer); err != nil {
			return "", fmt.Errorf("read secure random bytes: %w", err)
		}
		if buffer[0] >= 234 {
			continue
		}
		encoded = append(encoded, 'a'+buffer[0]%26)
	}
	return string(encoded), nil
}
