package tor

import (
	"context"
	"fmt"
	"net"
	"time"
	"golang.org/x/net/proxy"
)

const websiteTimeout = 60 * time.Second 

func (t *Tor)ConnectOnion(address string)(net.Conn, error){
	dialer, err := proxy.SOCKS5("tcp",t.SocksAddr,nil,proxy.Direct)
	if err != nil{
		return nil, fmt.Errorf("cannot reach the socks proxy at %s: %w", t.SocksAddr, err)
	}
	withCtx, ok := dialer.(proxy.ContextDialer)
	if !ok{
		return nil, fmt.Errorf("socks dialer cannot take a context")
	}
	ctx, cancel := context.WithTimeout(context.Background(), websiteTimeout)
	defer cancel()
	conn, err := withCtx.DialContext(ctx, "tcp", address)
	if err != nil{
		return nil, fmt.Errorf("cannot reach %s:%w", address, err)
	}
	return conn, nil
}
