package main

import (
	"fmt"
	"log"

	"github.com/jacob-cantrell/blog-aggregator/internal/config"
)

func main() {
	c, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	err = c.SetUser("jacob-cantrell")
	if err != nil {
		log.Fatal(err)
	}

	newConfig, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(newConfig.DBUrl)
}
