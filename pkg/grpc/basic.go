package grpc

import (
	"google.golang.org/grpc"
	"net"
	"yunka.io/pkg/logExt"
)

/**
* @Description: TODO
* @date 2019-07-26
* @version V1.0
 */
const (
	maxMsgSize = 200 * 1024 * 1024
)

func NewGrpcServer(opt ...grpc.ServerOption) *grpc.Server {
	return grpc.NewServer(append([]grpc.ServerOption{grpc.MaxRecvMsgSize(maxMsgSize), grpc.MaxSendMsgSize(maxMsgSize)},
		opt...)...)
}

func StartSrv(s *grpc.Server, ipPort string) error {
	logExt.Debug(ipPort)
	lis, err := net.Listen("tcp", ipPort)
	if err != nil {
		panic(err)
	}
	return s.Serve(lis)
}
