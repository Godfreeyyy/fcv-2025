package errs

import "errors"

var InvalidCredentials = errors.New("invalid credentials")

var (
	InternalError      = errors.New("internal error")
	GeneratingToken    = errors.New("error generating token")
	EmailRequired      = errors.New("email is required")
	ShouldUseFPTEmail  = errors.New("please use your FPT university email")
	FailedToCreateUser = errors.New("failed to create user")
)
