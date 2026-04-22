package pool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testItem struct {
	value      int
	resetCalls int
}

func (i *testItem) Reset() {
	i.value = 0
	i.resetCalls++
}

// TestGet_ReturnsZeroValueWhenEmpty проверяет, что пустой пул возвращает нулевое значение типа.
func TestGet_ReturnsZeroValueWhenEmpty(t *testing.T) {
	// Arrange
	newF := func() *testItem {
		return nil
	}
	p := New[*testItem](newF)

	// Act
	got := p.Get()

	// Assert
	require.Nil(t, got)
}

// TestPut_ResetsItemBeforeStoring проверяет, что при возврате в пул объект очищается через Reset.
func TestPut_ResetsItemBeforeStoring(t *testing.T) {
	// Arrange
	newF := func() *testItem {
		return nil
	}
	p := New[*testItem](newF)
	item := &testItem{value: 42}

	// Act
	p.Put(item)

	// Assert
	require.Zero(t, item.value)
	require.Equal(t, 1, item.resetCalls)
}

// TestGet_ReturnsZeroValueAfterDraining проверяет, что после извлечения всех элементов пул снова ведет себя как пустой.
func TestGet_ReturnsZeroValueAfterDraining(t *testing.T) {
	// Arrange
	newF := func() *testItem {
		return nil
	}
	p := New[*testItem](newF)
	p.Put(&testItem{value: 7})

	// Act
	_ = p.Get()
	got := p.Get()

	// Assert
	require.Nil(t, got)
}
