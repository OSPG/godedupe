package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"

	blake "github.com/minio/blake2b-simd"
)

const bufferSize = 2 * 1024

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
		for ; bufNumber > 0; bufNumber-- {
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

// ConvertBytes to convenient convert bytes to other units
func ConvertBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%v bytes", bytes)
	} else if bytes > 1024 && bytes < 1048576 {
		return fmt.Sprintf("%.2f Kb", float32(bytes)/float32(1024))
	} else if bytes > 1048576 && bytes < 1073741824 {
		return fmt.Sprintf("%.2f Mb", float32(bytes)/float32(1048576))
	}
	return fmt.Sprintf("%.2f Gb", float32(bytes)/float32(1073741824))
}
