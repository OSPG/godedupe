# godedupe

`godedupe` is a read-only duplicate finder for GNU/Linux. It scans one or more
directory trees and reports duplicate files and duplicate directory trees.

The program is designed for collections where hashing every file on every run
would be wasteful. It rejects unique files early, compares independent
candidate sets in parallel, and can persist computed hashes in a local SQLite
cache.

Currently `godedupe` only reports duplicates. It does not delete files, replace them with
links, or otherwise modify the scanned data.

It's capable of:

- Recognizing complete duplicate directory trees by their internal names,
  structure, file sizes, and contents.
- Processing independent candidate sets in parallel.
- Persists directory hashes and full hashes of large files in SQLite, allowing
  lightweight scheduled runs.
- Supports recursive scanning, depth limits, filename patterns, exclude files,
  hidden-file filtering, empty-file filtering, etc.
- Provides normal, one-set-per-line, summary, JSON, and desktop-notification
  output.

## Usage

Each target directory is supplied with `-t`. The option may be repeated.

```sh
./godedupe -t /data/photos
./godedupe -t /data/archive-a -t /data/archive-b
```

Show a summary of the reclaimable space:

```sh
./godedupe -m -t /data
```

Report only duplicate directories:

```sh
./godedupe --directories-only -t /data
```

Scan only JPEG files:

```sh
./godedupe -pattern '*.jpg' -t ~/Pictures
```

Produce compact output suitable for further processing:

```sh
./godedupe -1 -t /data
```

The Boolean options use Go's flag syntax. For example, recursion is disabled
with `-r=false`, and the cache is disabled with `--directory-cache=false`.

## Excluding paths

By default, `godedupe` tries to read `~/.godedupe-exclude`. If the default file
does not exist, the scan continues after printing a warning. An explicitly
selected exclude file must be readable.

Patterns support the usual filepath wildcards (`*`, `?`, and character
classes) plus `**`, which matches across directory levels. Prefix a pattern
with `!` to negate it. Patterns are matched against scanned paths.

Example:

```text
**/.cache/**
**/*.tmp
```

A directory containing a file named `.godedupe_ignore` is skipped as a whole.

## How duplicate detection works

### Files

File comparison has three stages:

1. Files are grouped by size. A size occurring once cannot be a duplicate and
   is discarded without reading the file contents.
2. Remaining files larger than 1 KiB are grouped by a CRC64 hash of their first
   1 KiB. Smaller files are hashed in full at this stage.
3. Sets that still contain multiple candidates are grouped by a full-file
   CRC64 hash.

Candidate sets are the unit of parallel work. Several sets can be compared at
once, but the files inside a set are read sequentially. This keeps parallelism
bounded and makes progress meaningful: `17 / 42 done` means that 17 complete
candidate sets have finished.

CRC64 is used as a fast checksum, not as a cryptographic digest.
`godedupe` currently treats equal full CRC64 hashes as equal content; it does
not perform a final byte-for-byte verification. This gives the tool a small
theoretical collision risk and should be considered before using its output 
to drive destructive automation.

### Directories

Directory comparison applies a similar staged approach to complete trees. It
first narrows candidates by file count and structure, then computes partial and
full content hashes. File and child-directory names are part of the directory
hash, so two trees must have the same layout as well as matching content.

Incomplete trees are not reported as duplicate directories. A tree is marked
incomplete when recursion is disabled or the depth limit prevents scanning a
child. Empty trees are also omitted.

After directory matches are known, nested duplicate directories are collapsed
and file matches covered by those directories are removed. The result describes
the largest useful duplicate units instead of repeating every file beneath
them.

## Persistent cache

The cache is enabled by default. It stores directory hashes and full hashes for
files of at least 500 MiB in SQLite. Cache keys include filesystem identity and
file metadata; changed entries are therefore recomputed instead of blindly
reused.

The default database is:

```text
~/.local/share/godedupe/directory-hashes.sqlite3
```

`XDG_DATA_HOME` is respected. `XDG_USER_DATA_DIR`, when set, is used directly
as the cache directory. Disable persistence for a run with:

```sh
./godedupe --directory-cache=false -t /data
```

## Compared with fdupes

Both tools reduce I/O by grouping files by size before doing content work.
[`fdupes`](https://github.com/adrianlopezroche/fdupes) is a mature
file-oriented utility: it compares size and MD5 signatures and then confirms
matches byte by byte. It also offers an optional signature database and actions
such as interactive deletion that `godedupe` deliberately does not provide.

`godedupe` is useful when the following differences matter:

- **Directory-level results.** A copied tree is reported as a duplicate
  directory rather than as a long list of unrelated file pairs.
- **Less redundant output.** Once a duplicate tree is reported, its contained
  files and nested duplicate trees are suppressed.
- **Directory-aware caching by default.** The SQLite cache covers complete
  directory hashes as well as full hashes for large files.
- **Set-level parallelism.** Independent candidate sets are processed across
  available CPUs, while progress is reported at the same set boundary.
- **Path filtering.** Recursive `**` exclusions and per-tree
  `.godedupe_ignore` files can remove irrelevant data before comparison.

These are workload-dependent improvements, not a claim that `godedupe` is
always faster. A first scan still has to read candidate data, storage throughput
often dominates runtime, and one very large candidate set cannot benefit from
set-level parallelism by itself. Conversely, `fdupes` has the stronger final
verification model because it performs a byte-for-byte comparison. Choose it
when collision-independent confirmation or built-in deletion is required.
