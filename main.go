package main

import (
	"github.com/koofr/graval"
	"github.com/journeymidnight/yig-ftp/s3adapter"
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
		PassiveOpts: &graval.PassiveOpts{
			ListenAddress: globalConfig.Host,
			NatAddress:    globalConfig.Host,
			PassivePorts: &graval.PassivePorts{
				Low:  42000,
				High: 45000,
			},
		},
	})

	log.Printf("FTP2S3 server listening on %s:%d", globalConfig.Host, globalConfig.Port)
	log.Printf("Access: ftp://%s:%d/", globalConfig.Host, globalConfig.Port)

	err := server.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}
}
