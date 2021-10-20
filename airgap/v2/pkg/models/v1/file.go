package modelv1

type File struct {
	ID      string `gorm:"primaryKey"`
	Content []byte
}

type FileMetadata struct {
	ID         string `gorm:"primaryKey"`
	MetadataID string
	Key        string `gorm:"index"`
	Value      string
}

type Metadata struct {
	ID                  string `gorm:"primaryKey"`
	ProvidedId          string
	ProvidedName        string
	Size                uint32
	Compression         bool
	CompressionType     string
	CreatedAt           int64
	DeletedAt           int64
	CleanTombstoneSetAt int64
	FileID              string
	File                File
	FileMetadata        []FileMetadata
	Checksum            string
}
