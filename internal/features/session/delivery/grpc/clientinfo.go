package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// clientInfo extracts the caller's user agent and IP address. Calls that
// arrive through grpc-gateway carry the originals in gateway metadata keys.
func clientInfo(ctx context.Context) (userAgent, ipAddress string) {
	md, _ := metadata.FromIncomingContext(ctx)

	userAgent = firstValue(md, "grpcgateway-user-agent", "user-agent")

	if forwarded := firstValue(md, "x-forwarded-for"); forwarded != "" {
		ipAddress = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	} else if p, ok := peer.FromContext(ctx); ok {
		ipAddress = p.Addr.String()
	}
	return userAgent, ipAddress
}

func firstValue(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		if vals := md.Get(key); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
