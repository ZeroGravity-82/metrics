package httpserver

import (
	"context"
	"net"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestPprofServer_Run_AddressInUse(t *testing.T) {
	// Arrange
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	logger := zerolog.Nop()
	srv := NewPprofServer(listener.Addr().String(), logger)
	ctx := context.Background()

	// Act
	err = srv.Run(ctx)

	// Assert
	require.Error(t, err)
}
