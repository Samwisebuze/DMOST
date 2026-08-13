package app

import (
	"github.com/samwisebuze/dmost/internal/infra/inmem"
	"github.com/samwisebuze/dmost/pkg/app/services"
)

type App struct {
	UserService UserService
}

func New() *App {
	return &App{
		UserService: services.NewUserService(inmem.NewUserRepository()),
	}
}
