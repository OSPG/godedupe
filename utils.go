package main

import (
	"bufio"
	"io"
	"os"
	"os/user"

	blake "github.com/minio/blake2b-simd"
)

// GetUserHome obtain the current home directory of the user
func GetUserHome() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	return usr.HomeDir
}

const bufferSize = 2 * 1024

// ComputeHash calculates the hash for the current file
// if bufferNumber is not zero then we will only hash the first bufferNumber blocks (bufferSize)
func ComputeHash(filename string, bufNumber int) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := blake.New256()
	buf, reader := make([]byte, bufferSize), bufio.NewReader(file)
	if bufNumber <= 0 {
		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}

			hash.Write(buf[:n])
		}
	} else {
		for ; bufNumber > 0; bufNumber -= 1 {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}

			hash.Write(buf[:n])
		}
	}

	return hash.Sum(nil), nil
}
