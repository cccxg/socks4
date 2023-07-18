package socks4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
)

var (
	Version4 byte = 0x04

	CmdConnect byte = 0x01
	CmdBind    byte = 0x02

	Granted byte = 0x5a // request granted.
	// request rejected or failure.
	RejectOrFailure byte = 0x5b
	// request rejected cause can't connect to identd on client.
	RejectNoIdentd byte = 0x5c
	// request rejected cause the user id reported by identd was different
	// from the user id reported by the request.
	RejectWrongUserId byte = 0x5d

	NullByte byte = 0x00
)

// A Request represents a SOCKS proxy request sent from the client.
type Request struct {
	Version byte   // SOCKS request version.
	Cmd     byte   // SOCKS request operation command.
	Port    int    // target host port.
	Address string // target host address, IP or domain name (SOCKS 4A).
	IsV4A   bool   // denote it is a SOCKS 4A request or not.
	UserId  string // the user id reported by client's request.
}

func ParseRequest(b []byte) (req Request, err error) {
	n := len(b)
	if n < 9 {
		err = errors.New("invalid SOCKS 4 request")
		return
	}

	if version := b[0]; version != Version4 {
		err = errors.New("invalid SOCKS request VN")
		return
	} else {
		req.Version = Version4
	}

	if cmd := b[1]; cmd != CmdConnect && cmd != CmdBind {
		err = errors.New("invalid SOCKS request CD")
		return
	} else {
		req.Cmd = cmd
	}

	req.Port = int(binary.BigEndian.Uint16(b[2:4]))

	// check SOCKS 4A
	if b[4] == 0 && b[5] == 0 && b[6] == 0 && b[7] != 0 {
		// SOCKS 4A
		req.IsV4A = true
		bs := bytes.Split(b[8:], []byte{NullByte})
		if len(bs) != 3 {
			err = errors.New("invalid SOCKS 4A request")
			return
		}
		req.UserId = string(bs[0])
		domainName := string(bs[1])
		req.Address = domainName + ":" + strconv.Itoa(req.Port)
	} else {
		// SOCKS 4
		req.IsV4A = false
		ip := net.IPv4(b[4], b[5], b[6], b[7]).String()
		req.Address = ip + ":" + strconv.Itoa(req.Port)
		req.UserId = string(b[8 : n-1]) // -1 for the last NULL byte.
	}

	return
}

// Reply represents a message that the SOCKS 4 server reply to the client's
// request.
type Reply struct {
	Cd   byte // the reply code.
	Port int
	IP   net.IP
}

func (r Reply) ToBytes() []byte {
	var b = []byte{0} // init with SOCKS version
	// add CD
	if r.Cd != Granted && r.Cd != RejectOrFailure &&
		r.Cd != RejectNoIdentd && r.Cd != RejectWrongUserId {
		b = append(b, RejectOrFailure)
	} else {
		b = append(b, r.Cd)
	}
	// add port
	b = binary.BigEndian.AppendUint16(b, uint16(r.Port))
	// add IP
	ip := r.IP.To4()
	b = append(b, ip...)
	return b
}
