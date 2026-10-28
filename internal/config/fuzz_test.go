package config

import "testing"

func FuzzValidateInput(f *testing.F) {
	f.Add("https://example.com/path?token=secret", "Authorization: Bearer secret")
	f.Add("http://[::1]:8080/", "X-Test: value")
	f.Add("https://user:pass@example.com/", "Bad\nHeader: value")
	f.Fuzz(func(t *testing.T, target, header string) {
		if len(target) > 16<<10 || len(header) > 16<<10 {
			t.Skip()
		}
		options := Defaults()
		_ = Validate(Input{Target: target, Headers: []string{header}, Options: options})
	})
}
