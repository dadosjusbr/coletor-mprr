package status

import (
	"errors"
	"fmt"
	"log"
	"os"
)

type Code int

const (
	Ok              Code = 0
	Unknown         Code = 1
	DataUnavailable Code = 4
)

// ExitFromError logs the error message and call os.Exit
// passing the code if err is of type StatusError
func ExitFromError(err error) {
	log.Println(fmt.Errorf("%q", err))
	var se *Error
	if errors.As(err, &se) {
		os.Exit(int(se.Code))
	}
	os.Exit(int(Unknown))
}
