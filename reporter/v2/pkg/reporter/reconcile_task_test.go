package reporter

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gotidy/ptr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	dataservicev1 "github.com/redhat-marketplace/redhat-marketplace-operator/airgap/v2/apis/dataservice/v1"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/dataservice"
	"github.com/redhat-marketplace/redhat-marketplace-operator/reporter/v2/pkg/uploaders"
	marketplacev1alpha1 "github.com/redhat-marketplace/redhat-marketplace-operator/v2/apis/marketplace/v1alpha1"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("reconcile_task", func() {
	var (
		ns  = "openshift-redhat-marketplace"
		ctx context.Context
		sut *ReconcileTask

		mockReportTask *mockTask

		meterReports   = []*marketplacev1alpha1.MeterReport{}
		isDisconnected = ptr.Bool(false)
	)

	BeforeEach(func() {
		var err error
		ctx = context.Background()

		mockReportTask = &mockTask{}

		sut, err = NewReconcileTask(ctx,
			&Config{
				K8sRestConfig: cfg,
				UploaderTargets: uploaders.UploaderTargets{
					&uploaders.NoOpUploader{},
				},
				IsDisconnected: *isDisconnected,
			},
			eb,
			Namespace(ns),
			func(ctx context.Context, reportName ReportName, taskConfig *Config) (TaskRun, error) {
				return mockReportTask, nil
			},
			func(ctx context.Context, config *Config, namespace Namespace) (UploadRun, error) {
				return mockReportTask, nil
			},
		)
		Expect(err).To(Succeed())
	})

	BeforeEach(func() {
		for i := 0; i < 5; i = i + 1 {
			report := marketplacev1alpha1.MeterReport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "report-ready-" + strconv.Itoa(i),
					Namespace: ns,
				},
				Spec: marketplacev1alpha1.MeterReportSpec{
					StartTime: metav1.NewTime(time.Now().Add(-3 * 24 * time.Hour)),
					EndTime:   metav1.NewTime(time.Now().Add(-2 * 24 * time.Hour)),
				},
				Status: marketplacev1alpha1.MeterReportStatus{},
			}
			Expect(k8sClient.Create(ctx, &report)).To(Succeed())
			meterReports = append(meterReports, &report)
		}
	})

	AfterEach(func() {
		Expect(k8sClient.DeleteAllOf(ctx, &marketplacev1alpha1.MeterReport{}, client.InNamespace(ns))).To(Succeed())
	})

	It("should query for meter reports and run them", func() {
		mockReportTask.On("Run", ctx).Return(nil).Times(5)
		mockReportTask.On("RunGeneric", ctx).Return(nil).Once()
		mockReportTask.On("RunReport", ctx, mock.IsType(&marketplacev1alpha1.MeterReport{})).Return(nil).Times(5)
		Expect(sut.report(ctx)).To(Succeed())

		for i := range meterReports {
			meterReports[i].Status.DataServiceStatus = &marketplacev1alpha1.UploadDetails{
				Target: uploaders.UploaderTargetDataService.Name(),
				ID:     fmt.Sprintf("%d", i),
				Status: marketplacev1alpha1.UploadStatusSuccess,
			}
			meterReports[i].Status.MetricUploadCount = ptr.Int(50)
			Expect(k8sClient.Status().Update(context.Background(), meterReports[i])).To(Succeed())
		}

		Expect(sut.upload(ctx)).To(Succeed())
		mockReportTask.AssertExpectations(GinkgoT())
	})

	It("should not run finished reports", func() {
		for i, report := range meterReports {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(report), report)).To(Succeed())
			report.Status.DataServiceStatus = &marketplacev1alpha1.UploadDetails{
				Target: uploaders.UploaderTargetDataService.Name(),
				ID:     fmt.Sprintf("%d", i),
				Status: marketplacev1alpha1.UploadStatusSuccess,
			}
			report.Status.UploadStatus = []*marketplacev1alpha1.UploadDetails{
				{
					Target: uploaders.UploaderTargetMarketplace.Name(),
					ID:     fmt.Sprintf("upload-%d", i),
					Status: marketplacev1alpha1.UploadStatusSuccess,
				},
			}
			Expect(k8sClient.Status().Update(ctx, report)).To(Succeed())
		}

		mockReportTask.On("RunGeneric", ctx).Return(nil).Once()
		Expect(sut.Run(ctx)).To(Succeed())
		mockReportTask.AssertExpectations(GinkgoT())
	})

	It("should not retry multiple times", func() {
		for i := range meterReports {
			report := *meterReports[i]
			Expect(sut.K8SClient.Get(ctx, client.ObjectKeyFromObject(&report), &report)).To(Succeed())
			report.Status.DataServiceStatus = &marketplacev1alpha1.UploadDetails{
				Target: uploaders.UploaderTargetDataService.Name(),
				ID:     fmt.Sprintf("%d", i),
				Status: marketplacev1alpha1.UploadStatusSuccess,
			}
			report.Status.UploadStatus = []*marketplacev1alpha1.UploadDetails{
				{
					Target: uploaders.UploaderTargetMarketplace.Name(),
					ID:     fmt.Sprintf("upload-%d", i),
					Status: marketplacev1alpha1.UploadStatusFailure,
				},
			}
			Expect(sut.K8SClient.Status().Update(ctx, &report)).To(Succeed())
		}
		mockReportTask.On("RunGeneric", ctx).Return(nil).Once()
		Expect(sut.upload(ctx)).To(Succeed())
		mockReportTask.AssertExpectations(GinkgoT())

		for i, report := range meterReports {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(report), report)).To(Succeed())
			cond := report.Status.Conditions.GetCondition(marketplacev1alpha1.ReportConditionTypeUploadStatus)
			Expect(cond).ToNot(BeNil(), fmt.Sprintf("%d", i))
			Expect(cond.Reason).To(Equal(marketplacev1alpha1.ReportConditionJobHasNoData.Reason), fmt.Sprintf("%d", i))
		}
	})

	Context("CanRunUploadReportTask", func() {
		var report = marketplacev1alpha1.MeterReport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "report",
				Namespace: ns,
			},
			Spec: marketplacev1alpha1.MeterReportSpec{
				StartTime: metav1.NewTime(time.Now().Add(-3 * 24 * time.Hour)),
				EndTime:   metav1.NewTime(time.Now().Add(-2 * 24 * time.Hour)),
			},
		}
		It("should filter properly", func() {
			reason, ok := sut.CanRunUploadReportTask(nil, report)
			Expect(ok).To(BeFalse())
			Expect(reason).To(Equal(SkipNoData))

			report.Status.DataServiceStatus = &marketplacev1alpha1.UploadDetails{
				Status: marketplacev1alpha1.UploadStatusSuccess,
			}
			reason, ok = sut.CanRunUploadReportTask(nil, report)
			Expect(ok).To(BeFalse())
			Expect(reason).To(Equal(SkipNoData))

			report.Status.MetricUploadCount = ptr.Int(2)

			reason, ok = sut.CanRunUploadReportTask(nil, report)
			Expect(ok).To(BeFalse())
			Expect(reason).To(Equal(SkipNoFileID))

			report.Status.DataServiceStatus.ID = "foo"
			sut.Config.IsDisconnected = true

			reason, ok = sut.CanRunUploadReportTask(nil, report)
			Expect(ok).To(BeFalse())
			Expect(reason).To(Equal(SkipDisconnected))

			sut.Config.IsDisconnected = false
			report.Status.RetryUpload = 3

			reason, ok = sut.CanRunUploadReportTask(nil, report)
			Expect(ok).To(BeFalse())
			Expect(reason).To(Equal(SkipMaxAttempts))

			report.Status.RetryUpload = 0

			reason, ok = sut.CanRunUploadReportTask(nil, report)
			Expect(ok).To(BeTrue())
		})
	})

})

type mockFileStorage struct {
	mock.Mock
}

var _ dataservice.FileStorage = &mockFileStorage{}

func (m *mockFileStorage) DownloadFile(ctx context.Context, f *dataservicev1.FileInfo) (file string, err error) {
	args := m.Called(ctx, f)
	return args.String(0), args.Error(1)
}
func (m *mockFileStorage) ListFiles(ctx context.Context) ([]*dataservicev1.FileInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*dataservicev1.FileInfo), args.Error(1)
}
func (m *mockFileStorage) GetFile(ctx context.Context, id string) (*dataservicev1.FileInfo, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*dataservicev1.FileInfo), args.Error(1)
}
func (m *mockFileStorage) Upload(ctx context.Context, f *dataservicev1.FileInfo, reader io.Reader) (id string, err error) {
	args := m.Called(ctx, f, reader)
	return args.String(0), args.Error(1)
}
func (m *mockFileStorage) UpdateMetadata(ctx context.Context, f *dataservicev1.FileInfo) (err error) {
	args := m.Called(ctx, f)
	return args.Error(0)
}
func (m *mockFileStorage) DeleteFile(ctx context.Context, f *dataservicev1.FileInfo) error {
	args := m.Called(ctx, f)
	return args.Error(0)
}

type mockTask struct {
	mock.Mock
}

func (m *mockTask) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockTask) RunGeneric(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockTask) RunReport(ctx context.Context, report *marketplacev1alpha1.MeterReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}
