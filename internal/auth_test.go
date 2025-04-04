package internal

import (
	"fmt"
	"regexp"
	"testing"
)

func Hello(name string) (string, error) {
	return fmt.Sprintf("Hello %s", name), nil
}
func TestHelloName(t *testing.T) {
	name := "Gladys"
	want := regexp.MustCompile(`\b` + name + `\b`)
	msg, err := Hello("Gladys")
	if !want.MatchString(msg) || err != nil {
		t.Fatalf(`Hello("Gladys") = %q, %v, want match for %#q, nil`, msg, err, want)
	}
}
