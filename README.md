# godedupe

Godedupe is a tool for finding duplicated or similar files.

It aims to be faster and provide a more intelligent approach for finding redundant files than old programs like fdupes.

The version 1.1.0 is our second version. In this version the only supported OS will be GNU/Linux and a lot of features are mising
but (at least) it is able to find duplicate files like fdupes does.

With the release of version 2.0.0 its planned to be able to find files which are almost identical. Useful for example when one file
is a modern version of other file.

### Version naming

We use an X.Y.Z style for naming our version. X means a new version that have some important new feature, Y means some minor feature
and Z means a correction.


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


### Resources

The images used are from [icons8](https://icons8.com/)
