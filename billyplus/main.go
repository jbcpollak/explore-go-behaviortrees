package main

import (
	"fmt"

	"github.com/billyplus/behavior"
	"github.com/billyplus/behavior/loader"
	"github.com/rs/zerolog/log"
)

func main() {
	fmt.Println("Hello, world.")
	behavior.Register("CalcAction", &CalcAction{})

	var bm, err = loader.LoadBeHaviorTreeFromFile("./test-project.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	var bt = bm.SelectBehaviorTree("mytree")
	if bt == nil {
		fmt.Println("tree not found")
		return
	}
	var bb = bt.NewBlackboard()
	var status = bt.Tick(bb)

	log.Info().Msgf("status: %v", status)
}
