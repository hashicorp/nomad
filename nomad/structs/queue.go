package structs

type QueueStatusRequest struct {
	QueryOptions
}

type DynamicPriorityStatus []DynamicPriorityWorkload

type DynamicPriorityWorkload struct {
	JobID            string
	Tenant           string
	AdjustedPriority int
	BasePriority     int
	UsageAjustment   int
	AgeAdjustment    int
	SizeAdjustment   int
}

type BatchQueueStatus any

type QueueStatusResponse struct {
	Type   BatchQueueType
	Status BatchQueueStatus
	QueryMeta
}
