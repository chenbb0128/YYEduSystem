package workers

import "github.com/hibiken/asynq"

func NewMux() *asynq.ServeMux {
	return asynq.NewServeMux()
}
