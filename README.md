# godedupe

Godedupe is a tool for finding duplicated or similar files.

It aims to be faster and provide a more intelligent approach for finding redundant files than old programs like fdupes.

The version 1.0.0 is our first version. In this version the only supported OS will be GNU/Linux and a lot of features are mising
but (at least) it is able to find duplicate files like fdupes does. 

With the release of version 2.0.0 its planned to be able to find files which are almost identical. Useful for example when one file 
is a modern version of other file, in this case the BLAKE sum will not be the same, but our program should be able to identify it.

### Version naming

We use an X.Y.Z style for naming our version. X means a new version that have some important new feature, Y means some minor feature
and Z means a correction.


### How we find duplicated files

For now our approach is very dummy. We simply declare a map of `Duplicated` where `Duplicated` is an struct like
```go 
type Duplicated struct {
	list_duplicated []File
}
```

Then we hash with the BLAKE algorithm the file that we want to test and check if this hash is already in the map, if it's true we append the File we just
tested to the `list_duplicated` and report that there is a duplicated, if it's false we create a new `Duplicated` and append the
 file we just checked to `list_duplicated`.