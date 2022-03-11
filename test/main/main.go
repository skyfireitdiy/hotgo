package main

import (
	"fmt"
	"time"

	"github.com/skyfireitdiy/hotgo/hotgo"
)

func oldFunc() {
	fmt.Println("old func")
}

func main() {
	ticker := time.NewTicker(time.Second)
	go func() {
		for range ticker.C {
			oldFunc()
		}
	}()

	for {
		input := ""
		fmt.Scanln(&input)
		if input == "exit" {
			break
		}
		if input == "load" {
			_, err := hotgo.Patch("./hp.so", hotgo.Config{
				ReplaceConfigs: []hotgo.ReplaceConfig{
					{
						OldSym: "main.oldFunc",
						NewSym: "NewFunc",
					},
				},
				RefConfigs: []hotgo.RefConfig{},
			})
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
