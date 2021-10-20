package database

import (
	"fmt"
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var _ = Describe("filestore", func() {
	var (
		db     *gorm.DB
		sut    StoredFileStore
		closer io.Closer
	)

	BeforeEach(func() {
		os.Remove("filestore.gorm.db")
		db, _ = gorm.Open(sqlite.Open("filestore.gorm.db"), &gorm.Config{})
		Expect(Migrate(db)).To(Succeed())
		sut, closer = New(db, FileStoreConfig{CleanupAfter: time.Duration(0)})
	})
	AfterEach(func() {
		Expect(closer.Close()).To(Succeed())
		os.Remove("filestore.gorm.db")
	})

	It("should save, update, soft delete and cleanup", func() {
		file := modelsv2.StoredFile{
			Name:       "foo.txt",
			Source:     "redhat-marketplace",
			SourceType: "report",
			File: modelsv2.StoredFileContent{
				Content:  []byte("test"),
				MimeType: "text/plain",
			},
			FileMetadata: []modelsv2.StoredFileMetadata{
				{
					Key:   "intervalStart",
					Value: fmt.Sprintf("%s", time.Now()),
				},
			},
		}
		id, err := sut.Save(&file)
		Expect(err).To(Succeed())
		Expect(id).ToNot(BeZero())

		file2, err := sut.Get(id)
		Expect(err).To(Succeed())

		Expect(file2.Name).To(Equal(file.Name))
		Expect(file2.File.Content).To(BeEmpty())
		Expect(file2.FileMetadata).To(HaveLen(1))

		file3, err := sut.Download(id)
		Expect(err).To(Succeed())

		Expect(file3.Name).To(Equal(file.Name))
		Expect(file3.File.Content).ToNot(BeEmpty())
		Expect(file3.FileMetadata).To(HaveLen(1))

		results, err := sut.List()
		Expect(results).To(HaveLen(1))

		err = sut.Delete(id)
		Expect(err).To(Succeed())

		files := []modelsv2.StoredFile{}
		Expect(db.Unscoped().Find(&files).Error).To(Succeed())
		Expect(files).To(HaveLen(1))

		content := []modelsv2.StoredFileContent{}
		Expect(db.Unscoped().Find(&content).Error).To(Succeed())
		Expect(content).To(HaveLen(1))
		Expect(content[0].DeletedAt.Valid).To(BeTrue())
		fmt.Println(content[0].DeletedAt.Time.UTC().String())

		results, err = sut.List()
		Expect(results).To(HaveLen(0))
		results, err = sut.List(ShowDeleted())
		Expect(results).To(HaveLen(1))

		affected, err := sut.CleanTombstones()
		Expect(err).To(Succeed())
		Expect(affected).To(Equal(int64(3)))

		files = []modelsv2.StoredFile{}
		Expect(db.Unscoped().Find(&files).Error).To(Succeed())
		Expect(files).To(HaveLen(0))

		content = []modelsv2.StoredFileContent{}
		Expect(db.Unscoped().Find(&content).Error).To(Succeed())
		Expect(content).To(HaveLen(0))

		metadata := []modelsv2.StoredFileMetadata{}
		Expect(db.Unscoped().Find(&metadata).Error).To(Succeed())
		Expect(metadata).To(HaveLen(0))
	})
})
