package main

import (
	"flag"
	"io"
	"net"
	"os"

	"github.com/notfilippo/mitm/proxy"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	localAddr   = flag.String("l", "0.0.0.0:3000", "local address")
	remoteAddr  = flag.String("r", "0.0.0.0:3000", "remote address")
	logFilePath = flag.String("log", "mitm.log", "output file path")
)

func main() {
	flag.Parse()

	file, err := os.OpenFile(*logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open file")
	}

	mw := io.MultiWriter(file, zerolog.ConsoleWriter{Out: os.Stderr})
	log.Logger = zerolog.New(mw).With().Timestamp().Logger()

	laddr, err := net.ResolveTCPAddr("tcp", *localAddr)
	if err != nil {
		log.Err(err).Msg("Failed to resolve local address")
		os.Exit(1)
	}
	raddr, err := net.ResolveTCPAddr("tcp", *remoteAddr)
	if err != nil {
		log.Err(err).Msg("Failed to resolve remote address")
		os.Exit(1)
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		log.Err(err).Msg("Failed to open local port to listen")
		os.Exit(1)
	}

	log.Info().
		Str("from", laddr.String()).
		Str("to", raddr.String()).
		Msg("Starting mitm")

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Err(err).Msg("Failed to accept connection")
			continue
		}

		var proxy *proxy.Proxy = proxy.New(conn, laddr, raddr)

		proxy.Matcher = func(bytes []byte, isLocal bool) {
			if isLocal {
				log.Info().Str("dir", "sent").Msg(string(bytes))
			} else {
				log.Info().Str("dir", "recv").Msg(string(bytes))
			}
		}

		go proxy.Start()
	}
}
