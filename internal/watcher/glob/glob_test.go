package glob

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{name: "empty matches empty", pattern: "", path: "", want: true},
		{name: "empty pattern rejects path", pattern: "", path: "file.c", want: false},
		{name: "literal", pattern: "main.c", path: "main.c", want: true},
		{name: "literal mismatch", pattern: "main.c", path: "main.cpp", want: false},
		{name: "star matches filename", pattern: "*.c", path: "main.c", want: true},
		{name: "star matches empty prefix", pattern: "*.c", path: ".c", want: true},
		{name: "star rejects slash", pattern: "*.c", path: "src/main.c", want: false},
		{name: "star matches many characters", pattern: "a*c", path: "abbbc", want: true},
		{name: "star matches no characters", pattern: "a*c", path: "ac", want: true},
		{name: "star does not cross slash", pattern: "a*c", path: "a/bc", want: false},
		{name: "question matches one character", pattern: "a?c", path: "abc", want: true},
		{name: "question rejects empty", pattern: "a?c", path: "ac", want: false},
		{name: "question rejects multiple", pattern: "a?c", path: "abbc", want: false},
		{name: "question does not match slash", pattern: "a?c", path: "a/c", want: false},
		{name: "doublestar matches empty", pattern: "**", path: "", want: true},
		{name: "doublestar matches nested path", pattern: "**", path: "one/two", want: true},
		{name: "doublestar extension root", pattern: "**/*.c", path: "main.c", want: true},
		{name: "doublestar extension one level", pattern: "**/*.c", path: "src/main.c", want: true},
		{name: "doublestar extension many levels", pattern: "**/*.c", path: "src/lib/main.c", want: true},
		{name: "doublestar extension mismatch", pattern: "**/*.c", path: "src/main.h", want: false},
		{name: "build subtree", pattern: "build/**", path: "build/app.o", want: true},
		{name: "build nested subtree", pattern: "build/**", path: "build/obj/app.o", want: true},
		{name: "build prefix mismatch", pattern: "build/**", path: "builder/app.o", want: false},
		{name: "middle doublestar zero levels", pattern: "src/**/main.c", path: "src/main.c", want: true},
		{name: "middle doublestar one level", pattern: "src/**/main.c", path: "src/lib/main.c", want: true},
		{name: "middle doublestar several levels", pattern: "src/**/main.c", path: "src/a/b/main.c", want: true},
		{name: "middle doublestar wrong name", pattern: "src/**/main.c", path: "src/lib/other.c", want: false},
		{name: "positive character class first", pattern: "file[abc].c", path: "filea.c", want: true},
		{name: "positive character class last", pattern: "file[abc].c", path: "filec.c", want: true},
		{name: "positive character class reject", pattern: "file[abc].c", path: "filed.c", want: false},
		{name: "character range low", pattern: "file[0-9].c", path: "file0.c", want: true},
		{name: "character range high", pattern: "file[0-9].c", path: "file9.c", want: true},
		{name: "character range reject", pattern: "file[0-9].c", path: "filex.c", want: false},
		{name: "negated class bang", pattern: "file[!a].c", path: "fileb.c", want: true},
		{name: "negated class bang reject", pattern: "file[!a].c", path: "filea.c", want: false},
		{name: "negated class caret", pattern: "file[^a].c", path: "fileb.c", want: true},
		{name: "class prefix with star", pattern: "[a-z]*.c", path: "hello.c", want: true},
		{name: "class does not match slash", pattern: "[a-z]/file.c", path: "/file.c", want: false},
		{name: "single directory star", pattern: "src/*.c", path: "src/main.c", want: true},
		{name: "single directory star rejects nested", pattern: "src/*.c", path: "src/lib/main.c", want: false},
		{name: "question in filename", pattern: "src/?ile.c", path: "src/file.c", want: true},
		{name: "question filename wrong length", pattern: "src/?ile.c", path: "src/afile.c", want: false},
		{name: "Windows path input", pattern: `src\**\*.cpp`, path: `src\app\main.cpp`, want: true},
		{name: "Windows root path input", pattern: `**\*.cpp`, path: `main.cpp`, want: true},
		{name: "Windows backslash in path", pattern: "src/*.h", path: `src\header.h`, want: true},
		{name: "backslash in pattern", pattern: `src\*.h`, path: "src/header.h", want: true},
		{name: "regex metacharacters are literal", pattern: "a+b.c", path: "a+b.c", want: true},
		{name: "regex metacharacters do not expand", pattern: "a+b.c", path: "aaabxc", want: false},
		{name: "malformed class", pattern: "file[abc.c", path: "filea.c", want: false},
		{name: "empty class", pattern: "file[].c", path: "file.c", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Match(test.pattern, test.path); got != test.want {
				t.Errorf("Match(%q, %q) = %t, want %t", test.pattern, test.path, got, test.want)
			}
		})
	}
}
