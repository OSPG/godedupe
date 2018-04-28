package main

import (
	"bufio"
	"io"
	"os"
	"os/user"

	"hash/crc64"
)

const bufferSize = 1024

// GetUserHome obtain the current home directory of the user
func GetUserHome() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	return usr.HomeDir
}

// ComputeHash calculates the hash for the current file
// if bufferNumber is not zero then we will only hash the first bufferNumber
// blocks (bufferSize)
func ComputeHash(filename string, bufNumber int) (uint64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	hash := crc64.New(crc64.MakeTable(crc64.ECMA))
	buf, reader := make([]byte, bufferSize), bufio.NewReader(file)
	if bufNumber <= 0 {
		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return 0, err
			}

			hash.Write(buf[:n])
		}
	} else {
		for ; bufNumber > 0; bufNumber-- {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return 0, err
			}

			hash.Write(buf[:n])
		}
	}

	return hash.Sum64(), nil
}
