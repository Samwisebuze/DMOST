package v1alpha

type Collection[T any] struct {
	Data  []T `json:"data"`
	Count int `json:"count"`
}
