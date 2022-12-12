package proxy

import (
	"io"
	"net"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Proxy - Manages a Proxy connection, piping data between local and remote.
type Proxy struct {
	laddr, raddr *net.TCPAddr
	lconn, rconn io.ReadWriteCloser
	erred        bool
	errsig       chan error

	Matcher  func([]byte, bool)
	Replacer func([]byte, bool) []byte

	// Settings
	Log        zerolog.Logger
	BufferSize int
}

// New - Create a new Proxy instance. Takes over local connection passed in,
// and closes it when finished.
func New(lconn *net.TCPConn, laddr, raddr *net.TCPAddr) *Proxy {
	return &Proxy{
		lconn:      lconn,
		laddr:      laddr,
		raddr:      raddr,
		erred:      false,
		errsig:     make(chan error),
		BufferSize: 0xFFFF,
	}
}

// Start - open connection to remote and start proxying data.
func (proxy *Proxy) Start() error {
	defer proxy.lconn.Close()

	var err error
	// connect to remote
	proxy.rconn, err = net.DialTCP("tcp", nil, proxy.raddr)
	if err != nil {
		proxy.Log.Err(err).Msg("Remote connection failed")
		return err
	}
	defer proxy.rconn.Close()

	// display both ends
	proxy.Log.Info().
		Str("from", proxy.laddr.String()).
		Str("to", proxy.raddr.String()).
		Msg("Opened connection")

	// bidirectional copy
	go proxy.pipe(proxy.lconn, proxy.rconn)
	go proxy.pipe(proxy.rconn, proxy.lconn)

	// wait for close...
	err = <-proxy.errsig
	proxy.Log.Info().Msg("Closed connection")

	return err
}

func (proxy *Proxy) err(s string, err error) {
	if proxy.erred {
		return
	}
	if err != io.EOF {
		log.Err(err).Msg(s)
	}
	proxy.errsig <- err
	proxy.erred = true
}

func (proxy *Proxy) pipe(src, dst io.ReadWriter) {
	// directional copy (64k buffer)
	buff := make([]byte, proxy.BufferSize)
	for {
		n, err := src.Read(buff)
		if err != nil {
			proxy.err("Read failed", err)
			return
		}
		b := buff[:n]

		isLocal := src == proxy.lconn

		// execute match
		if proxy.Matcher != nil {
			proxy.Matcher(b, isLocal)
		}

		// execute replace
		if proxy.Replacer != nil {
			b = proxy.Replacer(b, isLocal)
		}

		// write out result
		_, err = dst.Write(b)
		if err != nil {
			proxy.err("Write failed", err)
			return
		}
	}
}
