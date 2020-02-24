package s3adapter

import (
	"path/filepath"
	"github.com/yob/graval"
	"strings"
	"github.com/journeymidnight/aws-sdk-go/aws"
	"github.com/journeymidnight/aws-sdk-go/aws/credentials"
	"github.com/journeymidnight/aws-sdk-go/aws/session"
	"github.com/journeymidnight/aws-sdk-go/service/s3"
	"fmt"
	"time"
	"os"
	"io"
	"mime"
	"log"
	"bytes"
)

const PartLength = 5 << 20

type S3Driver struct {
	AWSRegion          string
	AWSBucketName      string
	AWSEndpoint        string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	WorkingDirectory   string
}

func (d *S3Driver) s3service() *s3.S3 {
	creds := credentials.NewStaticCredentials(d.AWSAccessKeyID, d.AWSSecretAccessKey, "")

	// By default make sure a region is specified
	s3client := s3.New(session.Must(session.NewSession(
		&aws.Config{
			Credentials: creds,
			DisableSSL:  aws.Bool(true),
			Endpoint:    aws.String(d.AWSEndpoint),
			Region:      aws.String("none"),
		},
	),
	),
	)
	return s3client
}

func pathToS3PathPrefix(path string) *string {
	path = strings.Replace(path, string(os.PathSeparator), "/", -1)
	path = strings.TrimPrefix(path, "/")

	if path == "" || strings.HasSuffix(path, "/") {
		return aws.String(path)
	}

	p := string(path) + "/"
	return aws.String(p)
}

func (d *S3Driver) s3DirContents(path string, maxKeys int64, marker string) (*s3.ListObjectsOutput, error) {
	svc := d.s3service()

	prefix := pathToS3PathPrefix(path)

	params := &s3.ListObjectsInput{
		Bucket:    aws.String(d.AWSBucketName), // Required
		Delimiter: aws.String(d.WorkingDirectory),
		// EncodingType: aws.String("EncodingType"),
		// Marker:       aws.String("Marker"),
		MaxKeys: aws.Int64(maxKeys),
		Prefix:  prefix,
	}

	if marker != "" {
		params.Marker = aws.String(marker)
	}

	resp, err := svc.ListObjects(params)

	if err != nil {
		// A service error occurred.
		fmt.Println("ListObjects Error: ", err)
	} else if err != nil {
		// A non-service error occurred.
		panic(err)
	}

	return resp, err
}

// Authenticate checks that the FTP username and password match
func (d *S3Driver) Authenticate(username string, password string) bool {
	sp := strings.Split(username, "/")
	if len(sp) != 2 {
		return false
	}
	d.AWSAccessKeyID = sp[0]
	d.AWSBucketName = sp[1]
	d.AWSSecretAccessKey = password

	svc := d.s3service()
	params := &s3.HeadBucketInput{
		Bucket: aws.String(d.AWSBucketName), // Required
	}

	_, err := svc.HeadBucket(params)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

// Bytes returns the ContentLength for the path if the key exists
func (d *S3Driver) Bytes(path string) int64 {
	svc := d.s3service()
	path = strings.Replace(path, string(os.PathSeparator), "/", -1)
	path = strings.TrimPrefix(path, "/")

	params := &s3.HeadObjectInput{
		Bucket: aws.String(d.AWSBucketName), // Required
		Key:    aws.String(path),            // Required
	}
	resp, err := svc.HeadObject(params)

	if err != nil {
		// A service error occurred.
		fmt.Println("HeadObject Error: ", err)
		return -1
	}

	return *resp.ContentLength
}

// ModifiedTime returns the LastModifiedTime for the path if the key exists
func (d *S3Driver) ModifiedTime(path string) (time.Time, error) {
	svc := d.s3service()

	path = strings.Replace(path, string(os.PathSeparator), "/", -1)
	path = strings.TrimPrefix(path, "/")

	params := &s3.HeadObjectInput{
		Bucket: aws.String(d.AWSBucketName), // Required
		Key:    aws.String(path),            // Required
	}
	resp, err := svc.HeadObject(params)

	if err != nil {
		// A service error occurred.
		fmt.Println("HeadObject Error: ", err)
		return time.Now(), err
	}

	return *resp.LastModified, nil
}

// ChangeDir “changes directories” on S3 if there are files under the given path
func (d *S3Driver) ChangeDir(path string) bool {
	// resp, err := d.s3DirContents(path, 1, "")
	path = strings.Replace(path, string(os.PathSeparator), "/", -1)
	if strings.HasPrefix(path, "/") {
		d.WorkingDirectory = strings.TrimPrefix(path, "/")
	} else {
		if strings.HasSuffix(d.WorkingDirectory, "/") {
			d.WorkingDirectory = d.WorkingDirectory + path
		} else {
			d.WorkingDirectory = d.WorkingDirectory + "/" + path
		}
	}

	fmt.Println("PWD:", d.WorkingDirectory)
	return true

	//
	// if err == nil && len(resp.Contents) > 0 {
	// 	d.WorkingDirectory = strings.TrimPrefix(path, "/")
	// 	return true
	// } else {
	// 	return false
	// }
}

// DirContents lists “directory” contents on S3
func (d *S3Driver) DirContents(path string) ([]os.FileInfo) {
	moreObjects := true
	var objects []*s3.Object

	var resp *s3.ListObjectsOutput
	var err error
	marker := ""

	for moreObjects {
		resp, err = d.s3DirContents(path, 1000, marker)

		if err == nil {
			for _, obj := range resp.Contents {
				objects = append(objects, obj)
			}

			moreObjects = *resp.IsTruncated

			if moreObjects {
				last := objects[len(objects)-1]
				marker = *last.Key
			}
		}
	}

	prefix := pathToS3PathPrefix(path)
	var files []os.FileInfo
	var dirs []string

	for _, object := range objects {
		p := *object.Key

		p = strings.TrimPrefix(p, *prefix)
		var fi os.FileInfo

		if strings.Contains(p, "/") || p == "" {
			parts := strings.Split(p, "/")
			dirPart := parts[0]

			if dirPart != d.WorkingDirectory && dirPart != "" && dirPart != "/" && !stringInSlice(dirPart, dirs) {
				fi = graval.NewDirItem(dirPart, time.Now().UTC())
				files = append(files, fi)

				dirs = append(dirs, dirPart)
			}
		} else {
			fi = graval.NewFileItem(p, *object.Size, *object.LastModified)
			files = append(files, fi)
		}
	}

	return files
}

// DeleteDir would delete a directory, but isn't currently implemented
func (d *S3Driver) DeleteDir(path string) bool {
        d.DeleteFile(path+"/")
	return false
}

// DeleteFile deletes the files from the given path
func (d *S3Driver) DeleteFile(path string) bool {
	svc := d.s3service()
	path = strings.Replace(path, string(os.PathSeparator), "/", -1)
	path = strings.TrimPrefix(path, "/")

	params := &s3.DeleteObjectInput{
		Bucket: aws.String(d.AWSBucketName), // Required
		Key:    aws.String(path),            // Required
	}
	_, err := svc.DeleteObject(params)

	if err != nil {
		// A service error occurred.
		fmt.Println("DeleteObject Error: ", err)
		return false
	}

	return true
}

// Rename isn't supported directly on S3
func (d *S3Driver) Rename(fromPath string, toPath string) bool {
	return false
}

// MakeDir would normally make a directory but this isn't supported on S3
func (d *S3Driver) MakeDir(path string) bool {
	d.PutFile(path+"/", nil)

	return false
}

// GetFile returns a reader for the given path on S3
func (d *S3Driver) GetFile(path string) (io.ReadCloser, error) {
	svc := d.s3service()

	path = strings.Replace(path, string(os.PathSeparator), "/", -1)
	path = strings.TrimPrefix(path, "/")

	params := &s3.GetObjectInput{
		Bucket: aws.String(d.AWSBucketName), // Required
		Key:    aws.String(path),            // Required
	}
	resp, err := svc.GetObject(params)
	if err != nil {
		// A service error occurred.
		fmt.Println("GetObject Error: ", err)
		return nil, err
	}

	return resp.Body, nil
}

// PutFile uploads a file to S3
func (d *S3Driver) PutFile(path string, reader io.Reader) bool {
	svc := d.s3service()
	path = strings.Replace(path, string(os.PathSeparator), "/", -1)
	if strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(path, string(os.PathSeparator))
	} else {
		path = d.WorkingDirectory + path
	}

	fileExt := filepath.Ext(path)

	contentType := mime.TypeByExtension(fileExt)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if strings.HasSuffix(path, "/") {
		var body io.ReadSeeker
		if reader != nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(reader)
			body = bytes.NewReader(buf.Bytes())
		}
		param := &s3.PutObjectInput{
			Bucket:      aws.String(d.AWSBucketName), // Required
			Key:         aws.String(path),            // Required
			Body:        body,
			ContentType: aws.String(contentType),
		}
		_, err := svc.PutObject(param)
		if err != nil {
			fmt.Println("Make dir error:", err)
			return false
		}
		return true
	}

	params := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(d.AWSBucketName),
		Key:         aws.String(path),
		ContentType: aws.String(contentType),
	}
	out, err := svc.CreateMultipartUpload(params)
	if err != nil {
		fmt.Println("Error: ", err)
		return false
	}
	uploadId := *out.UploadId

	var partNum int64 = 1
	offset := 0
	var etags []string
	var buf = make([]byte, PartLength)
	for {
		n, err := reader.Read(buf[offset:])
		if err != nil && err != io.EOF {
			fmt.Println("Put file error: ", err)
			AbortMultiPartUpload(svc, d.AWSBucketName, path, uploadId)
			return false
		}
		offset += n
		if err == io.EOF {
			if offset != 0 {
				etag, err := UploadPart(svc, d.AWSBucketName, path, buf[:offset] , uploadId, partNum)
				if err != nil {
					fmt.Println("UploadPart error: ", err)
					AbortMultiPartUpload(svc, d.AWSBucketName, path, uploadId)
					return false
				}
				etags = append(etags, etag)
			}
			break
		}

		if offset < PartLength {
			continue
		}

		etag, err := UploadPart(svc, d.AWSBucketName, path, buf, uploadId, partNum)
		if err != nil {
			fmt.Println("UploadPart error: ", err)
			AbortMultiPartUpload(svc, d.AWSBucketName, path, uploadId)
			return false
		}
		buf = make([]byte, PartLength)
		offset = 0
		partNum ++
		etags = append(etags, etag)
	}

	completedUpload := &s3.CompletedMultipartUpload{
		Parts: make([]*s3.CompletedPart, len(etags)),
	}

	for i := 0; i < len(etags); i++ {
		completedUpload.Parts[i] = &s3.CompletedPart{
			ETag:       aws.String(etags[i]),
			PartNumber: aws.Int64(int64(i + 1)),
		}
	}


	input := &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(d.AWSBucketName), // Required
		Key:             aws.String(path),            // Required
		MultipartUpload: completedUpload,
		UploadId:        aws.String(uploadId),
	}

	if _, err = svc.CompleteMultipartUpload(input); err != nil {
		fmt.Println("CompleteMultipartUpload error: ", err)
		AbortMultiPartUpload(svc, d.AWSBucketName, path, uploadId)
		return false
	}

	return true
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func AbortMultiPartUpload(svc *s3.S3, bucketName, key, uploadId string) (err error) {
	params := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadId),
	}
	_, err = svc.AbortMultipartUpload(params)
	return
}

func UploadPart(svc *s3.S3, bucketName, key string, value []byte, uploadId string, partNumber int64) (etag string, err error) {
	params := &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(key),
		Body:       bytes.NewReader(value),
		PartNumber: aws.Int64(partNumber),
		UploadId:   aws.String(uploadId),
	}
	out, err := svc.UploadPart(params)
	if err != nil {
		return
	}
	return *out.ETag, nil
}
