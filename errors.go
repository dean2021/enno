package enno

import "errors"

var ErrMissingProvider = errors.New("enno: missing provider")

var ErrNilSession = errors.New("enno: nil session")

var ErrUnsupportedOption = errors.New("enno: unsupported request option")
