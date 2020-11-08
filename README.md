[![Go Report Card](https://goreportcard.com/badge/github.com/OSPG/godedupe)](https://goreportcard.com/report/github.com/OSPG/godedupe)

# godedupe

Godedupe is a tool for finding duplicated or similar files.

It aims to be faster and provide a more intelligent approach for finding redundant files than old programs like fdupes.

The version 1.x.x is our first usable version. For now the only supported OS is
GNU/Linux and some features are measing but it is able to do their job.

### Small usage example

[![asciicast](https://asciinema.org/a/371391.svg)](https://asciinema.org/a/371391)

### How we find duplicated files

For now our approach is very dummy. We simply declare 3 maps of `Duplicated` where `Duplicated` is an struct like 
```go
type Duplicated struct {
	listDuplicated []File
}
```
so we can have a key with multiple values.

Then first we create the first map using as the key the size of each file. Then we check how many entries in that map have only one value, and delete these keys (they are unique). 
Then, for the keys that still have multiple values, a partial hash is done and saved to another map using the hash as the key. From that map, the keys that don't have multiple values are deleted.
Finally, for all the values that are left we do the full hash of the file and add to another map. We delete the keys that have only one value, and then we report all the files that are left.

For the hash we use a crc64
