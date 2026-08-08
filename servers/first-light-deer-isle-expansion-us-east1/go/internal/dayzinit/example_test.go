package dayzinit_test

import (
	"errors"
	"log"

	"dzcli/internal/dayzinit"
)

func ExampleValidateFile() {
	err := dayzinit.ValidateFile(`C:\dayz-server\mpmissions\dayzOffline.chernarusplus\init.c`)
	if err != nil {
		var validationErr *dayzinit.ValidationError
		if errors.As(err, &validationErr) {
			for _, diagnostic := range validationErr.Diagnostics {
				log.Printf("%s:%d:%d [%s] %s", validationErr.Path, diagnostic.Span.Start.Line, diagnostic.Span.Start.Column, diagnostic.Code, diagnostic.Message)
			}
		}
		return
	}
}
