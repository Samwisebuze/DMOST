package app

import (
	"github.com/samwisebuze/dmost/pkg/app/services"
	"github.com/samwisebuze/dmost/pkg/inmem"
)

type App struct {
	UserService UserService
}

func New() *App {
	return &App{
		UserService: services.NewUserService(inmem.NewUserRepository()),
	}
}
