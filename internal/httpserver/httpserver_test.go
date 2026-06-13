package httpserver

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"zerogravity-82/metrics/internal/repository"
	"zerogravity-82/metrics/internal/service/audit"
)

func TestHTTPServer_Run_AddressInUse(t *testing.T) {
	// Arrange
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	logger := zerolog.Nop()
	memStorage := repository.NewMemStorage()
	publisher := audit.NewAsyncPublisher(logger)
	signatureKey := "secret"
	cryptoKeyPath := ""
	trustedSubnet := ""
	tlsConfig := &tls.Config{}

	srv := NewHTTPServer(
		listener.Addr().String(),
		tlsConfig,
		memStorage,
		publisher,
		signatureKey,
		cryptoKeyPath,
		trustedSubnet,
		logger,
	)
	ctx := context.Background()

	// Act
	err = srv.Run(ctx)

	// Assert
	require.Error(t, err)
}
