module github.com/samwisebuze/dmost/pkg/inmem

go 1.26.3

replace github.com/samwisebuze/dmost/pkg/domain => ../domain

require (
	github.com/google/uuid v1.6.0
	github.com/samwisebuze/dmost/pkg/domain v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
