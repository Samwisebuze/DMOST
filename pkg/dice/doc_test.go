package dice

import (
	"os"
	"strings"
	"testing"
)

func docText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func requireAll(t *testing.T, doc string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(doc, w) {
			t.Errorf("doc.go missing %q", w)
		}
	}
}

func TestDocGrammar(t *testing.T) {
	d := docText(t)
	requireAll(t, d, "EBNF", "::=", "d%", "dF", "@name", "extension", "function", "modifier", "label",
		"floor", "min", "max", "abs")
}

func TestDocDifferencesTable(t *testing.T) {
	d := docText(t)
	requireAll(t, d, "Avrae", "d20", "Roll20", "dh", "dl", "success", "grouped", "alias")
}

func TestDocUnsupported(t *testing.T) {
	d := docText(t)
	requireAll(t, d, "Unsupported", "sets", "decimal", "comparison", "%", "//", "ra", "mi", "ma",
		"comment", "!!", "!p", "ErrSyntax")
}

func TestDocLimitsAndErrors(t *testing.T) {
	d := docText(t)
	requireAll(t, d,
		"ParseWithLimits", "DefaultMaxDice", "DefaultMaxSides", "DefaultMaxRerolls",
		"DefaultMaxExplosions", "DefaultMaxNodes", "1000", "10000", "100", "256",
		"parse time", "roll time", "tie", "floor division", "overflow",
		"ErrKeepCount", "ErrSyntax", "ErrUnknownVariable", "ErrDivisionByZero",
		"ErrOverflow", "ErrLimitExceeded", "ErrTooManyDice")
}

func TestDocPackageComment(t *testing.T) {
	d := docText(t)
	if !strings.HasPrefix(d, "// Package dice") && !strings.HasPrefix(d, "/*\nPackage dice") {
		t.Error("doc.go must start with a package dice comment")
	}
	if len(strings.Split(d, "\n")) < 80 {
		t.Error("doc.go design record is too short")
	}
}
