package server

import "github.com/benedict2310/htmlctl/internal/names"

func validateResourceName(name string) error {
	return names.ValidateResourceName(name)
}
