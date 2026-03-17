package types

type StatusEventsRequest struct {
	Id      string `path:"id"`
	Include string `form:"include,optional"`
}
