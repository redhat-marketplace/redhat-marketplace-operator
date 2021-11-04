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
	"context"
	"fmt"
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ = Describe("filestore", func() {
	var (
		db     *gorm.DB
		sut    StoredFileStore
		closer io.Closer
		ctx    = context.Background()

		files []*modelsv2.StoredFile
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

	Context("Paginate", func() {
		BeforeEach(func() {
			files = make([]*modelsv2.StoredFile, 20)

			for i := 0; i < 40; i++ {
				file := modelsv2.StoredFile{
					Name:       fmt.Sprintf("foo-%d.txt", i),
					Source:     "redhat-marketplace",
					SourceType: "report",
					File: modelsv2.StoredFileContent{
						Content:  []byte(fmt.Sprintf("test-%d", i)),
						MimeType: "text/plain",
					},
					FileMetadata: []modelsv2.StoredFileMetadata{
						{
							Key:   "intervalStart",
							Value: fmt.Sprintf("%s", time.Now()),
						},
						{
							Key:   "foo",
							Value: fmt.Sprintf("test-%d", i),
						},
					},
				}

				db.Save(&file)
				files = append(files, &file)
			}
		})

		AfterEach(func() {
			for _, file := range files {
				db.Association(clause.Associations).Delete(&file)
			}
		})

		It("should paginate by default", func() {
			results, token, err := sut.List(context.Background())
			Expect(err).To(Succeed())
			Expect(token).To(Equal("2"))
			Expect(results).To(HaveLen(10))
		})

		It("should paginate", func() {
			results, token, err := sut.List(context.Background(), Paginate("", 10))
			Expect(err).To(Succeed())
			Expect(token).To(Equal("2"))

			Expect(results).To(HaveLen(10))

			results2, token, err := sut.List(context.Background(), Paginate(token, 10))
			Expect(err).To(Succeed())
			Expect(token).To(Equal("3"))

			Expect(results2).To(HaveLen(10))

			newResults := []interface{}{}

			for _, r := range results2 {
				newResults = append(newResults, r)
			}

			Expect(results).ToNot(ContainElements(newResults...))

			results3, token, err := sut.List(context.Background(), Paginate(token, 10))
			Expect(err).To(Succeed())
			Expect(token).To(Equal("4"))
			Expect(results3).To(HaveLen(10))

			results4, token, err := sut.List(context.Background(), Paginate(token, 10))
			Expect(err).To(Succeed())
			Expect(token).To(Equal(""))
			Expect(results4).To(HaveLen(10))

			lastResults, token, err := sut.List(context.Background(), Paginate("", 100))
			Expect(err).To(Succeed())
			Expect(token).To(Equal(""))
			Expect(lastResults).To(HaveLen(40))
		})
	})

	Context("crud", func() {
		var file modelsv2.StoredFile

		BeforeEach(func() {
			file = modelsv2.StoredFile{
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
		})

		AfterEach(func() {
			db.Unscoped().Association(clause.Associations).Delete(&file)
		})

		It("should save, update, soft delete and cleanup", func() {
			id, err := sut.Save(ctx, &file)
			Expect(err).To(Succeed())
			Expect(id).ToNot(BeZero())

			file2, err := sut.Get(ctx, id)
			Expect(err).To(Succeed())
			Expect(file2.Name).To(Equal(file.Name))
			Expect(file2.File).ToNot(BeNil())
			Expect(file2.File.Content).To(BeEmpty())
			Expect(file2.File.Checksum).ToNot(BeEmpty())
			Expect(file2.FileMetadata).To(HaveLen(1))

			keyFile, err := sut.GetByFileKey(ctx, &modelsv2.StoredFileKey{
				Name:       file2.Name,
				Source:     file2.Source,
				SourceType: file2.SourceType,
			})
			Expect(err).To(Succeed())
			Expect(keyFile).To(Equal(file2))

			dlFile, err := sut.Download(ctx, id)
			Expect(err).To(Succeed())
			Expect(dlFile.Name).To(Equal(file.Name))
			Expect(dlFile.File.Content).ToNot(BeEmpty())
			Expect(dlFile.File.Checksum).ToNot(BeEmpty())
			Expect(dlFile.FileMetadata).To(HaveLen(1))

			file3, err := sut.Download(ctx, id)
			Expect(err).To(Succeed())

			Expect(file3.Name).To(Equal(file.Name))
			Expect(file3.File).ToNot(BeNil())
			Expect(file3.File.Content).ToNot(BeEmpty())
			Expect(file3.FileMetadata).To(HaveLen(1))

			results, _, err := sut.List(ctx)

			err = sut.Delete(ctx, id, false)
			Expect(err).To(Succeed())

			files := []modelsv2.StoredFile{}
			Expect(db.Unscoped().Find(&files).Error).To(Succeed())
			Expect(files).To(HaveLen(1))

			content := []modelsv2.StoredFileContent{}
			Expect(db.Unscoped().Find(&content).Error).To(Succeed())
			Expect(content).To(HaveLen(1))
			Expect(content[0].DeletedAt.Valid).To(BeTrue())

			results, _, err = sut.List(ctx)
			Expect(results).To(HaveLen(0))
			results, _, err = sut.List(ctx, ShowDeleted())
			Expect(results).To(HaveLen(1))
			Expect(results[0].File.Checksum).ToNot(Equal(""))
			Expect(results[0].FileMetadata).To(HaveLen(1))

			affected, err := sut.CleanTombstones(ctx)
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

			id, err = sut.Save(ctx, &file)
			Expect(err).To(Succeed())
			Expect(id).ToNot(BeZero())

			deletedFile, err := sut.Get(ctx, id)
			Expect(err).To(Succeed())
			Expect(deletedFile).ToNot(BeNil())
			Expect(deletedFile.ID).To(Not(BeNumerically("==", 0)))

			deletedFile, err = sut.Download(ctx, id)
			Expect(err).To(Succeed())
			Expect(deletedFile).ToNot(BeNil())
			Expect(deletedFile.File.Content).ToNot(BeEmpty())
			Expect(deletedFile.ID).To(Not(BeNumerically("==", 0)))

			Expect(sut.Delete(ctx, id, true)).To(Succeed())
			results, _, err = sut.List(ctx, ShowDeleted())
			Expect(results).To(HaveLen(0))
		})

	})
})
