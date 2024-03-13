package behaviortree

import (
	bt "github.com/joeycumines/go-behaviortree"
	"github.com/rs/zerolog/log"
)

// debug is a decorator that logs the status and error returned by the wrapped tick function.
func Debug(label string, tick bt.Tick) bt.Tick {
	return func(children []bt.Node) (status bt.Status, err error) {
		status, err = tick(children)
		log.Info().Msgf("%s returned (%v, %v)\n", label, status, err)
		return
	}
}
