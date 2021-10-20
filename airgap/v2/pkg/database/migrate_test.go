package database

import (
	"os"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	modelsv1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v1"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var _ = Describe("filestore", func() {
	Context("migrate", func() {
		var db *gorm.DB
		BeforeEach(func() {
			db, _ = gorm.Open(sqlite.Open("migrate.gorm.db"), &gorm.Config{})
		})
		AfterEach(func() {
			os.Remove("migrate.gorm.db")
		})
		It("should migrate", func() {
			Expect(Migrate(db)).To(Succeed())
		})
		It("should migrate data", func() {
			m := migrator(db)

			Expect(m.MigrateTo("202109010000")).To(Succeed())

			// add some data

			testId := uuid.NewString()
			testId2 := uuid.NewString()
			testId3 := uuid.NewString()
			testId4 := uuid.NewString()

			err := db.Save(&modelsv1.File{
				ID:      testId4,
				Content: []byte("foo"),
			}).Error
			Expect(err).To(Succeed())
			err = db.Save(&modelsv1.Metadata{
				ID:           testId,
				ProvidedId:   "foo",
				ProvidedName: "foo.txt",
				Compression:  true,
				FileID:       testId4,
				FileMetadata: []modelsv1.FileMetadata{
					{
						ID:         testId2,
						MetadataID: testId,
						Key:        "baz",
						Value:      "bar",
					},
					{
						ID:         testId3,
						MetadataID: testId,
						Key:        "baz2",
						Value:      "bar2",
					},
				},
			}).Error

			Expect(err).To(Succeed())

			// migrate rest
			Expect(m.Migrate()).To(Succeed())

			// confirm data
			data := modelsv2.StoredFile{}
			Expect(db.Preload("File").Preload("FileMetadata").First(&data).Error).To(Succeed())
			Expect(data.Name).To(Equal("foo.txt"))
			Expect(data.File.Checksum).ToNot(BeEmpty())
			Expect(data.File.Size).ToNot(BeZero())
			Expect(data.File.Content).To(Equal([]byte("foo")))
			Expect(data.FileMetadata).To(HaveLen(2))
			Expect(data.FileMetadata[0].Key).To(Equal("baz"))
			Expect(data.FileMetadata[0].Value).To(Equal("bar"))
		})
	})
})
