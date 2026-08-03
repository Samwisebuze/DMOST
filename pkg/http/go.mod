module github.com/samwisebuze/dmost/pkg/http

go 1.26.3

require (
	github.com/gorilla/mux v1.8.1
	github.com/samwisebuze/dmost/pkg/app v0.0.0-00010101000000-000000000000
	github.com/samwisebuze/dmost/pkg/domain v0.0.0-00010101000000-000000000000
	github.com/samwisebuze/dmost/pkg/dto v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.11.1
	golang.org/x/crypto v0.54.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/samwisebuze/dmost/pkg/inmem v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/samwisebuze/dmost/pkg/domain => ../domain

replace github.com/samwisebuze/dmost/pkg/dto => ../dto

replace github.com/samwisebuze/dmost/pkg/inmem => ../inmem

replace github.com/samwisebuze/dmost/pkg/app => ../app
