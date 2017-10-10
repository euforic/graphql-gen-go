package main

import (
	"fmt"
	"os"

	"github.com/euforic/graphql-gen-go/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
