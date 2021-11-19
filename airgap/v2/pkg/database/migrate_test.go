// Copyright 2021 IBM Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package database

import (
	"fmt"
	"os"
	"strconv"

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
			os.Remove("migrate.gorm.db")
			db, _ = gorm.Open(sqlite.Open("migrate.gorm.db"), &gorm.Config{})
		})
		AfterEach(func() {
			//os.Remove("migrate.gorm.db")
		})
		It("should migrate", func() {
			Expect(Migrate(db)).To(Succeed())
		})
		It("should migrate data", func() {
			m := migrator(db)

			Expect(m.MigrateTo("202109010000")).To(Succeed())

			// add some data
			fileID := uuid.NewString()

			files := []*modelsv1.File{}
			metadata := []*modelsv1.Metadata{}

			for i := 0; i < 200; i++ {
				localFileID := fileID + "-" + strconv.Itoa(i)

				files = append(files, &modelsv1.File{
					ID:      localFileID,
					Content: []byte("foo" + strconv.Itoa(i)),
				})

				metadataID := uuid.NewString()

				metadata = append(metadata, &modelsv1.Metadata{
					ID:           metadataID,
					ProvidedId:   "foo",
					ProvidedName: fmt.Sprintf("foo%v.txt", i),
					Compression:  true,
					FileID:       localFileID,
					FileMetadata: []modelsv1.FileMetadata{
						{
							ID:         metadataID + "A",
							MetadataID: metadataID,
							Key:        "baz",
							Value:      "bar",
						},
						{
							ID:         metadataID + "B",
							MetadataID: metadataID,
							Key:        "baz2",
							Value:      "bar2",
						},
					},
				})

			}

			Expect(db.CreateInBatches(files, 20).Error).To(Succeed())
			Expect(db.CreateInBatches(metadata, 20).Error).To(Succeed())

			// migrate rest
			Expect(m.Migrate()).To(Succeed())

			// confirm data
			data := []*modelsv2.StoredFile{}
			Expect(db.Preload("File").Preload("Metadata").Find(&data).Error).To(Succeed())
			Expect(data).To(HaveLen(200))

			file := modelsv2.StoredFile{}
			Expect(db.Preload("File").Preload("Metadata").First(&file).Error).To(Succeed())
			Expect(file.File.Checksum).ToNot(BeEmpty())
			Expect(file.File.Size).ToNot(BeZero())
			Expect(file.File.Content).To(HavePrefix("foo"))
			Expect(file.Metadata).To(HaveLen(2))
			Expect(file.Metadata[0].Key).To(Equal("baz"))
			Expect(file.Metadata[0].Value).To(Equal("bar"))

			Expect(db.Migrator().HasTable("files")).To(BeFalse())
			Expect(db.Migrator().HasTable("metadata")).To(BeFalse())
			Expect(db.Migrator().HasTable("file_metadata")).To(BeFalse())
			Expect(db.Migrator().HasTable("file_contents")).To(BeFalse())
		})
	})
})
