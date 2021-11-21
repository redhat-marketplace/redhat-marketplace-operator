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
	"log"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1/fileserver"
	modelsv2 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/pkg/models/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
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
		writer := logger.Writer(log.New(GinkgoWriter, "gorm-", log.LstdFlags))
		log := logger.New(writer, logger.Config{
			LogLevel: logger.Error,
		})
		db, _ = gorm.Open(sqlite.Open("filestore.gorm.db"), &gorm.Config{
			Logger: log,
		})
		Expect(Migrate(db)).To(Succeed())
		sut, closer = New(db, FileStoreConfig{CleanupAfter: time.Duration(0)})
	})

	AfterEach(func() {
		Expect(closer.Close()).To(Succeed())
		//os.Remove("filestore.gorm.db")
	})

	Context("Paginate", func() {
		BeforeEach(func() {
			files = make([]*modelsv2.StoredFile, 20)

			for i := 0; i < 20; i++ {
				file := modelsv2.StoredFile{
					Name:       fmt.Sprintf("empty-%d.txt", i),
					Source:     "redhat-marketplace",
					SourceType: "report",
					File: modelsv2.StoredFileContent{
						Content:  []byte(fmt.Sprintf("test-%d", i)),
						MimeType: "text/plain",
					},
				}

				db.Save(&file)
				files = append(files, &file)
			}

			for i := 0; i < 20; i++ {
				file := modelsv2.StoredFile{
					Name:       fmt.Sprintf("foo-%d.txt", i),
					Source:     "redhat-marketplace",
					SourceType: "report",
					File: modelsv2.StoredFileContent{
						Content:  []byte(fmt.Sprintf("test-%d", i)),
						MimeType: "text/plain",
					},
					Metadata: []modelsv2.StoredFileMetadata{
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

			for i := 0; i < 10; i++ {
				file := modelsv2.StoredFile{
					Name:       fmt.Sprintf("foo-%d.txt", i),
					Source:     "test",
					SourceType: "report1",
					File: modelsv2.StoredFileContent{
						Content:  []byte(fmt.Sprintf("test-%d", i)),
						MimeType: "text/plain",
					},
					Metadata: []modelsv2.StoredFileMetadata{
						{
							Key:   "intervalStart",
							Value: fmt.Sprintf("%s", time.Now()),
						},
						{
							Key:   "foo",
							Value: fmt.Sprintf("test-%d", i),
						},
						{
							Key:   "foo2",
							Value: fmt.Sprintf("test-%d", i),
						},
					},
				}
				db.Save(&file)
				files = append(files, &file)
			}

			for i := 0; i < 10; i++ {
				file := modelsv2.StoredFile{
					Name:       fmt.Sprintf("foo-%d.txt", i),
					Source:     "test-2",
					SourceType: "report1",
					File: modelsv2.StoredFileContent{
						Content:  []byte(fmt.Sprintf("test-%d", i)),
						MimeType: "text/plain",
					},
					Metadata: []modelsv2.StoredFileMetadata{
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
			Expect(token).To(Equal("5"))
			Expect(results4).To(HaveLen(10))

			lastResults, token, err := sut.List(context.Background(), Paginate("", 100))
			Expect(err).To(Succeed())
			Expect(token).To(Equal(""))
			Expect(lastResults).To(HaveLen(60))
		})

		Context("should filter", func() {
			filters := fileserver.Filters{}

			It("should apply simple filter", func() {
				filters.UnmarshalText([]byte(`source == "test-2"`))
				lastResults, token, err := sut.List(context.Background(), ApplyFilters(filters))
				Expect(err).To(Succeed())
				Expect(token).To(Equal(""))
				Expect(lastResults).To(HaveLen(10))
				Expect(lastResults).To(MatchMetadata(Not(BeEmpty())))
			})

			It("should apply harder filter", func() {
				filters = fileserver.Filters{}
				filters.UnmarshalText([]byte(`sourceType == "report1" && source=="test-2"`))
				lastResults, token, err := sut.List(context.Background(), ApplyFilters(filters), Paginate("", 100))
				Expect(err).To(Succeed())
				Expect(token).To(Equal(""))
				Expect(lastResults).To(HaveLen(10))
				Expect(lastResults).To(MatchMetadata(Or(HaveLen(2), HaveLen(3))))
			})

			It("should apply an or", func() {
				filters = fileserver.Filters{}
				filters.UnmarshalText([]byte(`sourceType == "report1" || source=="test-2"`))
				lastResults, token, err := sut.List(context.Background(), ApplyFilters(filters), Paginate("", 100))
				Expect(err).To(Succeed())
				Expect(token).To(Equal(""))
				Expect(lastResults).To(HaveLen(20))
				Expect(lastResults).To(MatchMetadata(Or(HaveLen(2), HaveLen(3))))
			})

			It("should handle dates", func() {
				filters = fileserver.Filters{}
				t1 := time.Now().Add(-time.Hour).Format(time.RFC3339)
				filters.UnmarshalText([]byte(`createdAt > "` + t1 + `"`))
				lastResults, token, err := sut.List(context.Background(), ApplyFilters(filters), Paginate("", 100))
				Expect(err).To(Succeed())
				Expect(token).To(Equal(""))
				Expect(lastResults).To(HaveLen(60))

				filters = fileserver.Filters{}
				t1 = time.Now().Add(-time.Hour).Format(time.RFC3339)
				filters.UnmarshalText([]byte(`createdAt < "` + t1 + `"`))
				lastResults, token, err = sut.List(context.Background(), ApplyFilters(filters), Paginate("", 100))
				Expect(err).To(Succeed())
				Expect(token).To(Equal(""))
				Expect(lastResults).To(HaveLen(0))
			})

			It("should handle metadata", func() {
				filters = fileserver.Filters{}
				filters.UnmarshalText([]byte(`key == "foo"`))
				lastResults, token, err := sut.List(context.Background(), ApplyFilters(filters), Paginate("", 100))
				Expect(err).To(Succeed())
				Expect(token).To(Equal(""))
				Expect(lastResults).To(HaveLen(40))
				Expect(lastResults).To(MatchMetadata(Or(HaveLen(2), HaveLen(3))))

				filters = fileserver.Filters{}
				filters.UnmarshalText([]byte(`key == "foo" && value == "test-1"`))
				lastResults, token, err = sut.List(context.Background(), ApplyFilters(filters), Paginate("", 100))
				Expect(err).To(Succeed())
				Expect(token).To(Equal(""))
				Expect(lastResults).To(HaveLen(3))
				Expect(lastResults).To(MatchMetadata(Or(HaveLen(3), HaveLen(2))))
			})
		})
	})

	Context("saveOverwrite", func() {
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
				Metadata: []modelsv2.StoredFileMetadata{
					{
						Key:   "intervalStart",
						Value: fmt.Sprintf("%s", time.Now()),
					},
				},
			}

			id, err := sut.Save(ctx, &file)
			Expect(err).To(Succeed())
			Expect(id).ToNot(BeZero())
		})

		AfterEach(func() {
			db.Unscoped().Association(clause.Associations).Delete(&file)
		})

		It("should override with changes", func() {
			file2 := file
			file2.Model = gorm.Model{}
			file2.Metadata = []modelsv2.StoredFileMetadata{
				{
					Key:   "intervalStart",
					Value: fmt.Sprintf("%s", time.Now()),
				},
				{
					Key:   "intervalEnd",
					Value: fmt.Sprintf("%s", time.Now()),
				},
			}

			id, err := sut.Save(ctx, &file2)
			Expect(err).To(Succeed())
			Expect(id).ToNot(BeZero())
			sut.Delete(ctx, id, false)

			id, err = sut.Save(ctx, &file2)
			Expect(err).To(Succeed())
			Expect(id).ToNot(BeZero())

			file3, err := sut.Get(ctx, id)
			Expect(err).To(Succeed())
			Expect(file3.Metadata).To(HaveLen(2))
			Expect(file3.DeletedAt.Time.IsZero()).To(BeTrue())
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
				Metadata: []modelsv2.StoredFileMetadata{
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

			fileCopy := file
			fileCopy.ID = 0
			id2, err := sut.Save(ctx, &fileCopy)
			Expect(err).To(Succeed())
			Expect(id2).To(Equal(id))

			file2, err := sut.Get(ctx, id)
			Expect(err).To(Succeed())
			Expect(file2.Name).To(Equal(file.Name))
			Expect(file2.File).ToNot(BeNil())
			Expect(file2.File.Content).To(BeEmpty())
			Expect(file2.File.Checksum).ToNot(BeEmpty())
			Expect(file2).To(MatchMetadata(HaveLen(1)))

			proto, err := modelsv2.StoredFileToProto(file2)
			Expect(err).To(Succeed())
			Expect(proto.Metadata).To(HaveLen(1))
			Expect(proto.Checksum).To(Equal(file2.File.Checksum))

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
			Expect(dlFile).To(MatchMetadata(HaveLen(1)))

			file3, err := sut.Download(ctx, id)
			Expect(err).To(Succeed())

			Expect(file3.Name).To(Equal(file.Name))
			Expect(file3.File).ToNot(BeNil())
			Expect(file3.File.Content).ToNot(BeEmpty())
			Expect(file3.Metadata).To(HaveLen(1))
			Expect(file3).To(MatchMetadata(HaveLen(1)))

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

type hasMetadata struct {
	metadataMatcher types.GomegaMatcher
}

func (h hasMetadata) Match(actual interface{}) (success bool, err error) {
	if v, ok := actual.([]modelsv2.StoredFile); ok {
		for _, vr := range v {
			ok, err := MatchFields(IgnoreExtras, Fields{
				"Metadata": h.metadataMatcher,
			}).Match(vr)

			if !ok {
				return ok, err
			}
		}

		return true, nil
	}

	if v, ok := actual.([]modelsv2.StoredFile); ok {
		for _, vr := range v {
			ok, err := MatchFields(IgnoreExtras, Fields{
				"Metadata": h.metadataMatcher,
			}).Match(vr)

			if !ok {
				return ok, err
			}
		}

		return true, nil
	}

	if v, ok := actual.(*modelsv2.StoredFile); ok {
		return PointTo(MatchFields(IgnoreExtras, Fields{
			"Metadata": h.metadataMatcher,
		})).Match(v)
	}

	if v, ok := actual.(modelsv2.StoredFile); ok {
		return MatchFields(IgnoreExtras, Fields{
			"Metadata": h.metadataMatcher,
		}).Match(v)
	}

	return false, nil
}

func (h hasMetadata) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("metadata doesn't match %v", actual)
}

func (h hasMetadata) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("metadata does match %v", actual)
}

func MatchMetadata(matcher types.GomegaMatcher) types.GomegaMatcher {
	return hasMetadata{metadataMatcher: matcher}
}
