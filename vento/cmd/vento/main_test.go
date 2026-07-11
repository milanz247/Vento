package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jinzhu/inflection"
)

func TestStudlyCase(t *testing.T) {
	cases := map[string]string{
		"blog_post":  "BlogPost",
		"blog-post":  "BlogPost",
		"blogPost":   "BlogPost",
		"post":       "Post",
		"Post":       "Post",
		"":           "",
		"post_2fa":   "Post2fa",
		"---":        "",
		"already_ok": "AlreadyOk",
	}
	for in, want := range cases {
		if got := studlyCase(in); got != want {
			t.Errorf("studlyCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnakeCase(t *testing.T) {
	cases := map[string]string{
		"BlogPost": "blog_post",
		"Post":     "post",
		"ID":       "i_d",
		"":         "",
	}
	for in, want := range cases {
		if got := snakeCase(in); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLowerCamel(t *testing.T) {
	cases := map[string]string{
		"BlogPost": "blogPost",
		"Post":     "post",
		"":         "",
		"A":        "a",
	}
	for in, want := range cases {
		if got := lowerCamel(in); got != want {
			t.Errorf("lowerCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStudlyCaseThenSnakeCaseRoundTripsToLowerSnake(t *testing.T) {
	// The make: commands chain these two: snakeCase(studlyCase(raw)) is
	// what becomes a file name, so the round trip needs to stay stable for
	// already-snake_case input.
	cases := []string{"blog_post", "user", "api_token"}
	for _, in := range cases {
		got := snakeCase(studlyCase(in))
		if got != in {
			t.Errorf("snakeCase(studlyCase(%q)) = %q, want %q", in, got, in)
		}
	}
}

func TestIsValidDBName(t *testing.T) {
	cases := map[string]bool{
		"vento_app":     true,
		"VentoApp123":   true,
		"":              false,
		"vento-app":     false,
		"vento app":     false,
		"vento;DROP":    false,
		"vento_app; --": false,
		"`vento`":       false,
	}
	for in, want := range cases {
		if got := isValidDBName(in); got != want {
			t.Errorf("isValidDBName(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestScaffoldTemplatesProduceValidGo guards against the exact bug class a
// hand-edited Printf-style template invites: a stray literal %s/%d inside
// generated Go source (e.g. inside a t.Fatalf call) being swallowed or
// misinterpreted by fmt.Sprintf's own verb processing instead of being
// escaped as %%s/%%d, or a raw-string/backtick-concatenation mistake
// (resourceControllerStub embeds a literal backtick for a struct tag
// example via string concatenation) producing syntactically invalid Go.
// Every stub is rendered with representative arguments and parsed with
// go/parser - a corrupted template fails this test immediately instead of
// surfacing as "go build" failing on a file some other developer just
// generated.
func TestScaffoldTemplatesProduceValidGo(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"controllerStub", fmt.Sprintf(controllerStub, "Post", "posts")},
		{"resourceControllerStub", fmt.Sprintf(resourceControllerStub, "Post", "posts", "post")},
		{"resourceTestStub", fmt.Sprintf(resourceTestStub, "Post", "posts", "post")},
		{"modelStub", fmt.Sprintf(modelStub, "Post")},
		{"middlewareStub", fmt.Sprintf(middlewareStub, "RequireAuth")},
		{"migrationStub", fmt.Sprintf(migrationStub, "20260101_000000_create_posts_table")},
		{"seederStub", fmt.Sprintf(seederStub, "Post", "posts")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			if _, err := parser.ParseFile(fset, tc.name+".go", tc.src, parser.AllErrors); err != nil {
				t.Fatalf("generated source is not valid Go: %v\n--- source ---\n%s", err, tc.src)
			}
		})
	}
}

// TestScaffoldTemplatesHaveNoUnescapedPercent is a narrower, more direct
// check for the specific mistake TestScaffoldTemplatesProduceValidGo would
// only catch indirectly: every stub containing a literal Printf-style verb
// (resourceTestStub's t.Fatalf calls) must double its percent signs, or
// fmt.Sprintf silently consumes/misformats them instead of leaving the
// literal text alone.
func TestScaffoldTemplatesHaveNoUnescapedPercent(t *testing.T) {
	rendered := fmt.Sprintf(resourceTestStub, "Post", "posts", "post")
	if !strings.Contains(rendered, `%d`) || !strings.Contains(rendered, `%s`) {
		t.Fatalf("expected the rendered test stub to contain literal %%d/%%s verbs (from t.Fatalf calls), got:\n%s", rendered)
	}
}

// repoRoot returns the module root, three directories up from this test's
// package (vento/cmd/vento).
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join(".", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected %s to be the module root (go.mod not found): %v", root, err)
	}
	return root
}

// writeAndRemove writes content to path (relative to root), registers its
// removal via t.Cleanup so it's deleted even if the test fails partway
// through, and fails the test outright if path already exists - a
// collision would mean the throwaway name picked below isn't actually
// throwaway.
func writeAndRemove(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, relPath)
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("refusing to overwrite existing file %s - pick a different throwaway name", relPath)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating directory for %s: %v", relPath, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", relPath, err)
	}
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Errorf("cleaning up %s: %v", relPath, err)
		}
	})
}

// TestGeneratedResourceActuallyCompiles is the test that would have caught
// the seederStub bug this suite shipped once: seederStub imported
// "vento-app/app/models" unconditionally, but its generated body only
// referenced models.T inside a comment - an unused import, which
// go/parser's pure syntax check (TestScaffoldTemplatesProduceValidGo)
// cannot detect, since "unused import" is a type-checking error, not a
// parse error. This test writes every make:resource/make:seeder output
// into real locations under the actual module and runs `go build ./...`
// against it - the same check a developer's first `go build` after
// scaffolding would hit - then removes every file it created, pass or
// fail.
func TestGeneratedResourceActuallyCompiles(t *testing.T) {
	root := repoRoot(t)
	const name = "CliScaffoldCheck" // unique, unlikely to collide with a real resource

	lower := strings.ToLower(name)
	varName := lowerCamel(name)

	writeAndRemove(t, root, filepath.Join("app", "controllers", snakeCase(name)+"_controller.go"),
		fmt.Sprintf(resourceControllerStub, name, lower, varName))
	writeAndRemove(t, root, filepath.Join("app", "models", snakeCase(name)+".go"),
		fmt.Sprintf(modelStub, name))
	writeAndRemove(t, root, filepath.Join("app", "controllers", snakeCase(name)+"_controller_test.go"),
		fmt.Sprintf(resourceTestStub, name, lower, varName))
	writeAndRemove(t, root, filepath.Join("app", "seeders", snakeCase(name)+"_seeder.go"),
		fmt.Sprintf(seederStub, name, lower))
	writeAndRemove(t, root, filepath.Join("app", "middleware", snakeCase(name)+".go"),
		fmt.Sprintf(middlewareStub, name))
	writeAndRemove(t, root, filepath.Join("migrations", "99999999_999999_"+snakeCase(name)+".go"),
		fmt.Sprintf(migrationStub, "99999999_999999_"+snakeCase(name)))

	const simpleName = "CliScaffoldCheckSimple" // controllerStub writes to the same *_controller.go path as resourceControllerStub, so it needs a distinct name
	writeAndRemove(t, root, filepath.Join("app", "controllers", snakeCase(simpleName)+"_controller.go"),
		fmt.Sprintf(controllerStub, simpleName, inflection.Plural(strings.ToLower(simpleName))))

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated scaffold does not compile:\n%s", out)
	}
}
