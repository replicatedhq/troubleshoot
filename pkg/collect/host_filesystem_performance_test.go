package collect

import (
	"fmt"
	"testing"
)

func TestGetPercentileIndex(t *testing.T) {
	tests := []struct {
		length int
		p      float64
		answer int
	}{
		{
			length: 2,
			p:      0.49,
			answer: 0,
		},
		{
			length: 2,
			p:      0.5,
			answer: 0,
		},
		{
			length: 2,
			p:      0.51,
			answer: 1,
		},
		{
			length: 100,
			p:      0.01,
			answer: 0,
		},
		{
			length: 100,
			p:      0.99,
			answer: 98,
		},
		{
			length: 100,
			p:      0.995,
			answer: 99,
		},
		{
			length: 10000,
			p:      0.999,
			answer: 9989,
		},
	}
	for _, test := range tests {
		name := fmt.Sprintf("(%f, %d) == %d", test.p, test.length, test.answer)
		t.Run(name, func(t *testing.T) {
			output := getPercentileIndex(test.p, test.length)
			if output != test.answer {
				t.Errorf("Got %d, want %d", output, test.answer)
			}
		})
	}
}
