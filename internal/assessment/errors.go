package assessment

import "errors"

var (
	ErrInvalidSample   = errors.New("invalid sample")
	ErrEvidenceMissing = errors.New("evidence missing")
	ErrAuthorRequired  = errors.New("author is required")
	ErrInvalidPlan     = errors.New("invalid treatment plan")
)
