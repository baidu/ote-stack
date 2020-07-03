package server

import (
	cert "github.com/baidu/ote-stack/pkg/server/certificate"
	"github.com/baidu/ote-stack/pkg/server/handler"
)

type EdgeServer interface {
	StartServer(*ServerContext) error
	CheckValid(*ServerContext) error
}

type ServerContext struct {
	BindAddr     string
	BindPort     int
	ReadTimeout  int
	WriteTimeout int
	CaFile       string
	CertFile     string
	KeyFile      string

	StopChan   chan bool
	HandlerCtx *handler.HandlerContext
	CertCtx    *cert.CertContext
}
