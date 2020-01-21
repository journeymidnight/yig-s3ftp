package s3adapter

import (
	"github.com/yob/graval"
)

type S3DriverFactory struct {
	AWSRegion          string
	AWSEndpoint        string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
}

func (f *S3DriverFactory) NewDriver() (d graval.FTPDriver, err error) {
	return &S3Driver{
		AWSRegion:          f.AWSRegion,
		AWSEndpoint:        f.AWSEndpoint,
		AWSAccessKeyID:     f.AWSAccessKeyID,
		AWSSecretAccessKey: f.AWSSecretAccessKey,
	}, nil
}
