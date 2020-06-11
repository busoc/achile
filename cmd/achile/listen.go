package main

import (
	"crypto/tls"
	"crypto/x509"
	"net"

	"github.com/midbel/achile"
	"github.com/midbel/cli"
	"github.com/midbel/toml"
)

func runListen(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	cfg := struct {
		Addr    string
		Base    string
		Clients uint16 `toml:"client"`
		Cert    struct {
			Pem  string
			Key  string
			Root string
		} `toml:"certificate"`
	}{}
	if err := toml.DecodeFile(cmd.Flag.Arg(0), &cfg); err != nil {
		return err
	}
	s, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	defer s.Close()

	if cfg.Cert.Pem != "" {
		cert, err := tls.LoadX509KeyPair(cfg.Cert.Pem, cfg.Cert.Key)
		if err != nil {
			return err
		}
		var root *x509.CertPool
		if cfg.Cert.Root == "" {
			root, err = x509.SystemCertPool()
		} else {
			root = x509.NewCertPool()
		}
		if err != nil {
			return err
		}
		c := tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    root,
		}
		s = tls.NewListener(s, &c)
	}
	defer s.Close()
	for {
		c, err := s.Accept()
		if err != nil {
			return err
		}
		h, err := achile.NewHandler(c, cfg.Base)
		if err == nil {
			go h.Handle()
		} else {
			c.Close()
		}
	}
	return nil
}
