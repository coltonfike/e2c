package core

import "errors"

var (
	errEquivocatingBlocks      = errors.New("equivocation detected")
	errUnknownBlock            = errors.New("unknown block")
	errNonuniqueSignatures     = errors.New("signatures not all unique")
	errDifferentView           = errors.New("msg from different view")
	errDuplicateBlock          = errors.New("given duplicate block")
	errNotEnoughSignatures     = errors.New("not enough signatures")
	errInvalidBlock            = errors.New("invalid block on proposal")
	errInvalidBlockCertificate = errors.New("invalid block certificate")
	errInvalidValidates        = errors.New("invalid validates")
)
