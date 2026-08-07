package main

import "testing"

func TestPortaDeHealthcheck(t *testing.T) {
	cases := map[string]string{
		":8927":          ":8927",
		"0.0.0.0:9090":   ":9090",
		"[::]:8927":      ":8927",
		"sem-porta-aqui": ":8927",
	}
	for input, expected := range cases {
		if got := healthcheckPort(input); got != expected {
			t.Errorf("healthcheckPort(%q) = %q, expected %q", input, got, expected)
		}
	}
}
