package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSemaphore(t *testing.T) {
	// Arrange
	maxReq := 5

	// Act
	sem := NewSemaphore(maxReq)

	// Assert
	assert.Equal(t, maxReq, cap(sem.semaCh))
}

func TestAcquire(t *testing.T) {
	// Arrange
	sem := NewSemaphore(1)

	// Act
	sem.Acquire()

	// Assert
	select {
	case sem.semaCh <- struct{}{}:
		t.Error("Expected semaphore to be full, but it was not")
	default:
		// Expected path
	}
}

func TestRelease(t *testing.T) {
	// Arrange
	sem := NewSemaphore(1)
	sem.Acquire()

	// Act
	sem.Release()

	// Arrange
	select {
	case sem.semaCh <- struct{}{}:
		// Expected path
	default:
		t.Error("Expected semaphore to be empty, but it was not")
	}
}

func TestBlockingBehavior(t *testing.T) {
	// Arrange
	sem := NewSemaphore(1)
	sem.Acquire()
	done := make(chan bool)

	// Act
	go func() {
		sem.Acquire()
		done <- true
	}()

	// Assert
	select {
	case <-done:
		t.Error("Expected goroutine to block, but it did not")
	case <-time.After(100 * time.Millisecond):
		// Expected path
	}
	sem.Release()
	select {
	case <-done:
		// Expected path
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected goroutine to proceed, but it did not")
	}
}

func TestZeroCapacity(t *testing.T) {
	// Arrange
	sem := NewSemaphore(0)
	done := make(chan bool)

	// Act
	go func() {
		sem.Acquire()
		done <- true
	}()

	// Assert
	select {
	case <-done:
		t.Error("Expected goroutine to block indefinitely, but it did not")
	case <-time.After(100 * time.Millisecond):
		// Expected path
	}
}

func TestNegativeCapacity(t *testing.T) {
	// Arrange
	defer func() {
		// Assert
		assert.NotNil(t, recover())
	}()

	// Act
	NewSemaphore(-1)
}
