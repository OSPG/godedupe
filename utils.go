package main

import (
	"os/user"
)

func GetUserHome() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	return usr.HomeDir
}
