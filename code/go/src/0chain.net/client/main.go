package main

import (
	. "0chain.net/filechunk"
)

// sample usage
func main() {
	var fileinfo ChunkingFilebyShardsIntf = &FileInfo{DataShards: 10,
		ParShards: 6,
		OutDir:    "",
		File:      "test.txt"}

	fileinfo.ChunkingFilebyShards()
}
