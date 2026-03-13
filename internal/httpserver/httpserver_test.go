package httpserver

import (
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

	logger := zerolog.New(nil)
	memStorage := repository.NewMemStorage()
	publisher := audit.NewAsyncPublisher(logger)

	srv := NewHTTPServer(listener.Addr().String(), memStorage, publisher, "secret", logger)

	// Act
	err = srv.Run(logger)

	// Assert
	require.Error(t, err)
}
