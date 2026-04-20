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
	p := New[*testItem]()

	// Act
	got := p.Get()

	// Assert
	require.Nil(t, got)
}

// TestPut_ResetsItemBeforeStoring проверяет, что при возврате в пул объект очищается через Reset.
func TestPut_ResetsItemBeforeStoring(t *testing.T) {
	// Arrange
	p := New[*testItem]()
	item := &testItem{value: 42}

	// Act
	p.Put(item)

	// Assert
	require.Zero(t, item.value)
	require.Equal(t, 1, item.resetCalls)
}

// TestGet_ReturnsItemsInLIFOOrder проверяет, что пул извлекает элементы в порядке LIFO.
func TestGet_ReturnsItemsInLIFOOrder(t *testing.T) {
	// Arrange
	p := New[*testItem]()
	first := &testItem{value: 1}
	second := &testItem{value: 2}
	p.Put(first)
	p.Put(second)

	// Act
	gotFirst := p.Get()
	gotSecond := p.Get()

	// Assert
	require.Same(t, second, gotFirst)
	require.Same(t, first, gotSecond)
	require.Equal(t, 1, gotFirst.resetCalls)
	require.Equal(t, 1, gotSecond.resetCalls)
}

// TestGet_ReturnsZeroValueAfterDraining проверяет, что после извлечения всех элементов пул снова ведет себя как пустой.
func TestGet_ReturnsZeroValueAfterDraining(t *testing.T) {
	// Arrange
	p := New[*testItem]()
	p.Put(&testItem{value: 7})

	// Act
	_ = p.Get()
	got := p.Get()

	// Assert
	require.Nil(t, got)
}
