package uploaders

import (
	"context"
	"os"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/go-logr/logr"
	"github.com/redhat-marketplace/redhat-marketplace-operator/v2/pkg/utils"
	yaml "gopkg.in/yaml.v2"
	"gorm.io/gorm/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ Uploader = &COSS3Uploader{}

type COSS3UploaderConfig struct {
	ApiKey            string `json:"apiKey" yaml:"apiKey"`
	ServiceInstanceID string `json:"serviceInstanceID" yaml:"serviceInstanceID"`
	AuthEndpoint      string `json:"authEndpoint" yaml:"authEndpoint"`
	ServiceEndpoint   string `json:"serviceEndpoint" yaml:"serviceEndpoint"`
	Bucket            string `json:"bucket" yaml:"bucket"`
}

type COSS3Uploader struct {
	COSS3UploaderConfig
	uploader *s3manager.Uploader
}

func NewCOSS3Uploader(
	config *COSS3UploaderConfig,
) (Uploader, error) {

	conf := aws.NewConfig().
		WithEndpoint(config.ServiceEndpoint).
		WithCredentials(ibmiam.NewStaticCredentials(
			aws.NewConfig(),
			config.AuthEndpoint,
			config.ApiKey,
			config.ServiceInstanceID,
		)).WithS3ForcePathStyle(true)

	sess := session.Must(session.NewSession(conf))
	uploader := s3manager.NewUploader(sess)

	return &COSS3Uploader{
		COSS3UploaderConfig: *config,
		uploader:            uploader,
	}, nil
}

func (r *COSS3Uploader) UploadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	result, err := r.uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(r.COSS3UploaderConfig.Bucket),
		Key:         aws.String(filepath.Base(file.Name())),
		Body:        file,
		ContentType: aws.String("application/gzip"),
	})
	if err != nil {
		return errors.Wrap(err, "failed to upload file")
	}

	logger.Info("file uploaded", "url", result.Location)

	return nil
}

func provideCOSS3Config(
	ctx context.Context,
	cc ClientCommandRunner,
	deployedNamespace string,
	log logr.Logger,
) (*COSS3UploaderConfig, error) {
	secret := &corev1.Secret{}

	result, _ := cc.Do(ctx,
		GetAction(types.NamespacedName{
			Name:      utils.RHM_COS_UPLOADER_SECRET,
			Namespace: deployedNamespace,
		}, secret))
	if !result.Is(Continue) {
		return nil, result
	}

	configYamlBytes, ok := secret.Data["config.yaml"]
	if !ok {
		return nil, errors.New("rhm-cos-uploader-secret does not contain a config.yaml")
	}

	cosS3UploaderConfig := &COSS3UploaderConfig{}

	err := yaml.Unmarshal(configYamlBytes, &cosS3UploaderConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal rhm-cos-uploader-secret config.yaml")
	}

	return cosS3UploaderConfig, nil
}
