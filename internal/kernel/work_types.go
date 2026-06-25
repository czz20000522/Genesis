package kernel

import "genesis/internal/kernel/workregistry"

const (
	WorkStatusOpen     = workregistry.StatusOpen
	WorkStatusCanceled = workregistry.StatusCanceled
)

type WorkSubmitRequest = workregistry.SubmitRequest
type WorkCancelRequest = workregistry.CancelRequest
type WorkProjection = workregistry.WorkProjection
