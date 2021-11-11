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
			Expect(db.Preload("File").Preload("Metadata").First(&data).Error).To(Succeed())
			Expect(data.Name).To(Equal("foo.txt"))
			Expect(data.File.Checksum).ToNot(BeEmpty())
			Expect(data.File.Size).ToNot(BeZero())
			Expect(data.File.Content).To(Equal([]byte("foo")))
			Expect(data.Metadata).To(HaveLen(2))
			Expect(data.Metadata[0].Key).To(Equal("baz"))
			Expect(data.Metadata[0].Value).To(Equal("bar"))
		})
	})
})
