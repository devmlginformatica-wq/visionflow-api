package apperrors

import (
	"errors"
	"fmt"
)

var ErrInvalidInput = errors.New("requisição inválida")
var ErrDuplicateResource = errors.New("recurso duplicado")
var ErrIntegrationLocked = errors.New("cadastro bloqueado pela integração ERP")

func InvalidInput(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, message)
}
