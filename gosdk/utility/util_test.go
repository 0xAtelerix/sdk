package utility

import (
	"bytes"
	"pgregory.net/rapid"
	"testing"
)

func TestFlattenUnflattenIdentity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Генерируем случайный [][]byte
		b := rapid.SliceOf(
			rapid.SliceOf(rapid.Byte()),
		).Draw(t, "data")

		// Сериализуем и десериализуем
		flat := Flatten(b)
		restored, err := Unflatten(flat)

		if err != nil {
			t.Fatalf("Unflatten returned error: %v", err)
		}

		// Проверка на равенство (глубокое сравнение)
		if !equalChunks(b, restored) {
			t.Fatalf("restored data not equal to original.\nOriginal: %v\nRestored: %v", b, restored)
		}
	})
}

// Вспомогательная функция для сравнения [][]byte
func equalChunks(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
