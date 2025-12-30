package utility

import (
	"bytes"
	"testing"

	"pgregory.net/rapid"
)

func TestFlattenUnflattenIdentity(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(tr *rapid.T) {
		t.Run(tr.Name(), func(_ *testing.T) {
			// Генерируем случайный [][]byte
			b := rapid.SliceOf(
				rapid.SliceOf(rapid.Byte()),
			).Draw(tr, "data")

			// Сериализуем и десериализуем
			flat, err := Flatten(b)
			if err != nil {
				tr.Fatalf("flatten returned error: %v", err)
			}

			restored, err := Unflatten(flat)
			if err != nil {
				tr.Fatalf("Unflatten returned error: %v", err)
			}

			// Проверка на равенство (глубокое сравнение)
			if !equalChunks(b, restored) {
				tr.Fatalf(
					"restored data not equal to original.\nOriginal: %v\nRestored: %v",
					b,
					restored,
				)
			}
		})
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
