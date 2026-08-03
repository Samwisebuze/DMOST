module github.com/samwisebuze/dmost/pkg/app

go 1.26.3

replace github.com/samwisebuze/dmost/pkg/domain => ../domain

replace github.com/samwisebuze/dmost/pkg/dto => ../dto

replace github.com/samwisebuze/dmost/pkg/inmem => ../inmem

require (
	github.com/samwisebuze/dmost/pkg/domain v0.0.0-00010101000000-000000000000
	github.com/samwisebuze/dmost/pkg/dto v0.0.0-00010101000000-000000000000
	github.com/samwisebuze/dmost/pkg/inmem v0.0.0-00010101000000-000000000000
)

require github.com/google/uuid v1.6.0 // indirect
