package main

import (
	"github.com/yob/graval"
	"github.com/journeymidnight/yig-s3ftp/s3adapter"
	"log"
)

func main() {
	ParseConfig()

	factory := &s3adapter.S3DriverFactory{
			AWSEndpoint:        globalConfig.Endpoint,
	}

	server := graval.NewFTPServer(&graval.FTPServerOpts{
		ServerName: "yig-ftp",
		Factory:    factory,
		Hostname:   globalConfig.Host,
		Port:       globalConfig.Port,
		PasvMinPort: 42000,
		PasvMaxPort: 45000,
	})

	log.Printf("FTP2S3 server listening on %s:%d", globalConfig.Host, globalConfig.Port)
	log.Printf("Access: ftp://%s:%d/", globalConfig.Host, globalConfig.Port)

	err := server.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}
}
