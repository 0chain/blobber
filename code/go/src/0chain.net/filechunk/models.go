package filechunk

//FileInfo struct
type FileInfo struct {
	DataShards int
	ParShards  int
	File       string
	OutDir     string
}

//Maininfo is for main response
type Maininfo struct {
	ID   string     `json:"id"`
	Meta []Metainfo `json:"meta"`
}

type Metainfo struct {
	Filename     string     `json:"filename"`
	Custom       CustomMeta `json:"custom_meta"`
	Size         int        `json:"size"`
	Content_hash string     `json:"content_hash"`
	// MetaCustom   *Custom_meta
}

type CustomMeta struct {
	PartNum int `json:"part_num"`
}
