package main

import (
	"os/user"
)

// GetUserHome obtain the current home directory of the user
func GetUserHome() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	return usr.HomeDir
}
