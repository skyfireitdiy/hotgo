package main

import (
	"fmt"
	"time"

	"github.com/skyfireitdiy/hotgo/hotgo"
)

var global int = 100

func oldFunc() {
	fmt.Printf("old func: %d\n", global)
}

func hiddenFunc() {
	fmt.Printf("hidden func: %d\n", global)
}

func main() {
	ticker := time.NewTicker(time.Second)
	hiddenFunc()
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
				RefConfigs: []hotgo.RefConfig{
					{
						RefSym:   "main.global",
						PatchSym: "Data",
					},
					{
						RefSym:   "main.hiddenFunc",
						PatchSym: "NewHiddenFunc",
					},
				},
			})
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}
