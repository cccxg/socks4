package socks4

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type OptionFunc func(*Server)

func WithLogger(logger Logger) OptionFunc {
	return func(s *Server) {
		s.logger = logger
	}
}

// Server implements a SOCKS 4 proxy server, which also support SOCKS 4A.
type Server struct {
	logger Logger
	lis    net.Listener
	wg     sync.WaitGroup
	closed bool
}

// NewServer creates and return a SOCKS 4 proxy server with given options.
// i.e.:
//
//	s := socks4.NewServer(WithLogger(customLogger))
func NewServer(opts ...OptionFunc) *Server {
	srv := &Server{}
	for _, opt := range opts {
		opt(srv)
	}

	if srv.logger == nil {
		srv.logger = &logrus.Logger{
			Out: os.Stdout,
			Formatter: &logrus.TextFormatter{
				TimestampFormat: time.DateTime,
			},
			Level: logrus.DebugLevel,
		}
	}

	return srv
}

// Run starts the SOCKS proxy server listening on given address.
// i.e.:
//
//	s := socks4.NewServer()
//	s.Run(":1080")
func (s *Server) Run(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	s.lis = lis
	s.closed = false
	s.wg = sync.WaitGroup{}
	defer lis.Close()
	s.logger.Infof("SOCKS server listen on %v", address)

	for {
		conn, err := lis.Accept()
		if err != nil {
			if s.closed {
				break
			}
			s.logger.Warnf("listener accept error: %v", err)
			continue
		}
		s.logger.Infof("accept connection from: %v", conn.RemoteAddr())
		s.wg.Add(1)
		go s.handleConn(conn)
	}

	return errors.New("listencer closed")
}

// ShutDown shut down the SOCKS server. The server will stop accepting
// new connections and wait for existing connections to complete.
func (s *Server) ShutDown() error {
	if s.lis == nil {
		return errors.New("can't shut down a server that has not been started")
	}
	s.closed = true
	if err := s.lis.Close(); err != nil {
		return err
	}
	s.logger.Info("server is shut down, waiting for existing connections to complete")
	s.wg.Wait()
	s.logger.Info("all connections are complete")
	return nil
}

// HandleConn handles connect from client.
func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	defer s.wg.Done()

	remote, err := s.establishProxy(conn)
	if err != nil {
		s.logger.Warnf("establish proxy error: %v", err)
		return
	}
	defer remote.Close()

	s.logger.Infof("proxy conn for client %v to target %v established", conn.RemoteAddr(), remote.RemoteAddr())
	s.transfer(conn, remote)
}

// establishProxy establishes a TCP connection with remote host.
func (s *Server) establishProxy(conn net.Conn) (net.Conn, error) {
	b := make([]byte, 41)
	n, err := conn.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read from connect: %v", err)
	}
	s.logger.Debugf("read request from client %v: %v", conn.RemoteAddr().String(), b[:n])
	req, err := ParseRequest(b[:n])
	if err != nil {
		return nil, err
	}

	var remote net.Conn
	if req.Cmd == CmdConnect {
		remote, err = s.establishConnect(conn, req)
		if err != nil {
			_, wErr := conn.Write(Reply{Cd: RejectOrFailure}.ToBytes())
			if err != nil {
				remote.Close()
				return nil, fmt.Errorf("failed to reply to client: %v", wErr)
			}
			return nil, fmt.Errorf("failed to establish connect for CONNECT request: %v", err)
		}
	} else if req.Cmd == CmdBind {
		remote, err = s.establishBind(conn, req)
		if err != nil {
			_, wErr := conn.Write(Reply{Cd: RejectOrFailure}.ToBytes())
			if wErr != nil {
				remote.Close()
				return nil, fmt.Errorf("failed to reply to client: %v", wErr)
			}
			return nil, fmt.Errorf("failed to establish connect for BIND request: %v", err)
		}
	} else {
		return nil, fmt.Errorf("unexpected error: got a request with operation command %v", req.Cmd)
	}

	local := remote.LocalAddr().String()
	addr, err := net.ResolveTCPAddr("tcp", local)
	if err != nil {
		remote.Close()
		return nil, err
	}

	rep := Reply{
		Cd:   Granted,
		Port: addr.Port,
		IP:   net.ParseIP(addr.IP.String()).To4(),
	}.ToBytes()
	_, err = conn.Write(rep)
	if err != nil {
		remote.Close()
		return nil, err
	}
	return remote, nil
}

// establishConnect establishes a TCP connection to remote host for
// SOCKS 4/4A CONNECT request.
func (s *Server) establishConnect(conn net.Conn, req Request) (net.Conn, error) {
	remote, err := net.Dial("tcp", req.Address)
	if err != nil {
		return nil, err
	}

	return remote, nil
}

// establishBind establishes an inbound TCP connection from remote host
// for SOCKS 4/4A BIND request.
func (s *Server) establishBind(conn net.Conn, req Request) (net.Conn, error) {
	lis, err := net.Listen("tcp", "")
	if err != nil {
		return nil, err
	}
	defer lis.Close()

	addr, err := net.ResolveTCPAddr("tcp", lis.Addr().String())
	if err != nil {
		return nil, err
	}

	// first reply
	if _, err := conn.Write(Reply{Cd: Granted, Port: addr.Port}.ToBytes()); err != nil {
		return nil, err
	}

	// max time for listening remote.
	go func() {
		time.Sleep(120 * time.Second)
		lis.Close()
	}()

	remote, err := lis.Accept()
	if err != nil {
		return nil, err
	}

	// TODO.
	// Normally, it should check wether the IP, port of remote host are
	// the same as the DST IP and DST port in the request.

	return remote, nil
}

// transfer relays data between client and remote host.
func (s *Server) transfer(client, remote net.Conn) {
	cliAddr, remoteAddr := client.RemoteAddr().String(), remote.RemoteAddr().String()
	s.logger.Infof("begin transfer data between client %v and remote host %v", cliAddr, remoteAddr)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(client, remote)
		wg.Done()
	}()
	go func() {
		io.Copy(remote, client)
		wg.Done()
	}()

	wg.Wait()
	s.logger.Infof("stop transfer data between client %v and remote host %v", cliAddr, remoteAddr)
}
