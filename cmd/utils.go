package cmd

import "github.com/google/uuid"

func parseToken(rawToken string) (uuid.UUID, error) {
	return uuid.Parse(rawToken)
}
