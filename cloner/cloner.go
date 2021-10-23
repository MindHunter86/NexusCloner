package app

import (
	"github.com/rs/zerolog"
	"gopkg.in/urfave/cli.v1"
)

type Cloner struct{}

var gLog *zerolog.Logger

func NewCloner(l *zerolog.Logger) *Cloner {
	gLog = l
	return &Cloner{}
}

func (m *Cloner) Bootstrap(ctx *cli.Context) error {
	return nil
}
