//go:build ignore

// setup.go is Vento's zero-config installer. It is excluded from normal
// builds by the ignore tag above; run it directly:
//
//	go run setup.go        (interactive)
//	go run setup.go 1      (non-interactive: local install)
//	go run setup.go 2      (non-interactive: global install)
//
// Option 1 (local, recommended) compiles the CLI to ./bin/vento (or
// bin\vento.exe on Windows) inside the project root - it cannot live at
// the root itself, where the name would collide with the vento/ framework
// package directory. Option 2 installs it globally: /usr/local/bin/vento
// on macOS/Linux, or a user-level PATH registration on Windows.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	reset  = "\033[0m"
	green  = "\033[32m"
	red    = "\033[31m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	bold   = "\033[1m"
)

func main() {
	fmt.Println(cyan + bold + "Vento setup" + reset + " - zero-config installer")
	fmt.Println()
	fmt.Println("  1) Local  (recommended): build the vento CLI into this project root")
	fmt.Println("  2) Global: install the vento CLI system-wide")
	fmt.Println()

	choice := ""
	if len(os.Args) > 1 {
		choice = os.Args[1]
	} else {
		fmt.Print("Choose an option [1]: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		choice = strings.TrimSpace(line)
	}
	if choice == "" {
		choice = "1"
	}

	binName := "vento"
	if runtime.GOOS == "windows" {
		binName = "vento.exe"
	}

	switch choice {
	case "1":
		installLocal(binName)
	case "2":
		installGlobal(binName)
	default:
		fmt.Fprintln(os.Stderr, red+"error:"+reset+" unknown option "+choice)
		os.Exit(1)
	}

	buildCSS()
}

// buildCSS runs the one-time Tailwind CSS build (npm install, then npm run
// build:css) so the welcome page renders styled immediately - without this,
// public/css/app.css never gets created and every page loads unstyled. This
// is best-effort: a missing npm/Node.js install shouldn't fail the whole
// setup, since the Go CLI itself doesn't need it.
func buildCSS() {
	if _, err := exec.LookPath("npm"); err != nil {
		fmt.Println(yellow + "npm not found in PATH" + reset + ` - skipping the CSS build. Install Node.js, then run:

    npm install && npm run build:css`)
		return
	}

	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		fmt.Println("installing frontend dependencies (npm install) ...")
		if err := runNpm("install"); err != nil {
			fmt.Fprintln(os.Stderr, red+"warning:"+reset+" npm install failed: "+err.Error())
			return
		}
	}

	fmt.Println("building Tailwind CSS (npm run build:css) ...")
	if err := runNpm("run", "build:css"); err != nil {
		fmt.Fprintln(os.Stderr, red+"warning:"+reset+" CSS build failed: "+err.Error())
		return
	}
	fmt.Println(green + "done!" + reset + " public/css/app.css built")
}

func runNpm(args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// build compiles vento/cmd/vento to the given output path.
func build(output string) {
	fmt.Printf("building %s ...\n", output)
	cmd := exec.Command("go", "build", "-o", output, "./vento/cmd/vento")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, red+"error:"+reset+" build failed: "+err.Error())
		os.Exit(1)
	}
}

// installLocal builds the CLI into ./bin inside the project root ("vento"
// itself is taken by the framework package directory).
func installLocal(binName string) {
	if err := os.MkdirAll("bin", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, red+"error:"+reset+" "+err.Error())
		os.Exit(1)
	}
	out := filepath.Join("bin", binName)
	build("./" + out)
	fmt.Println(green + "done!" + reset + " run the CLI with:")
	fmt.Println()
	fmt.Println("    ./" + filepath.ToSlash(out))
}

// installGlobal builds the CLI and registers it system-wide. On
// macOS/Linux it lands in /usr/local/bin (already on PATH, no profile
// edits needed); on Windows the project's bin directory is appended to
// the user-level PATH directly in the registry.
func installGlobal(binName string) {
	switch runtime.GOOS {
	case "windows":
		if err := os.MkdirAll("bin", 0o755); err != nil {
			fmt.Fprintln(os.Stderr, red+"error:"+reset+" "+err.Error())
			os.Exit(1)
		}
		build("./" + filepath.Join("bin", binName))
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, red+"error:"+reset+" "+err.Error())
			os.Exit(1)
		}
		binDir := filepath.Join(dir, "bin")
		// Append this project's bin directory to the user-level PATH so the
		// freshly built vento.exe resolves from any terminal. This must be
		// done via `reg add` against HKCU\Environment rather than `setx`:
		// setx silently truncates the value it writes to 1024 characters,
		// which both drops the appended directory and corrupts the rest of
		// the user's PATH if the combined string is longer (a well-known
		// setx footgun). We also only read/write the user-level PATH, not
		// the merged system+user PATH, to keep the value as short as
		// possible and avoid touching machine-wide settings.
		userPath := getRegUserPath()
		if strings.EqualFold(userPath, "") {
			userPath = binDir
		} else if !containsPathEntry(userPath, binDir) {
			userPath = userPath + ";" + binDir
		}
		cmd := exec.Command("reg", "add", `HKCU\Environment`, "/v", "PATH", "/t", "REG_EXPAND_SZ", "/d", userPath, "/f")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, red+"error:"+reset+" updating PATH failed: "+err.Error())
			os.Exit(1)
		}
		fmt.Println(green + "done!" + reset + " open a NEW terminal and run: vento")

	default: // macOS / Linux
		tmp := filepath.Join(os.TempDir(), binName)
		build(tmp)
		target := "/usr/local/bin/vento"
		if err := copyFile(tmp, target, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, red+"error:"+reset+" could not write "+target+": "+err.Error())
			fmt.Fprintln(os.Stderr, "try:  sudo cp "+tmp+" "+target)
			os.Exit(1)
		}
		fmt.Println(green + "done!" + reset + " installed to " + target + " - run: vento")
	}
}

// getRegUserPath reads the user-level PATH directly from the registry
// (HKCU\Environment), rather than os.Getenv("PATH") which returns the
// process's merged system+user PATH and would make the value we write
// back needlessly long.
func getRegUserPath() string {
	out, err := exec.Command("reg", "query", `HKCU\Environment`, "/v", "PATH").Output()
	if err != nil {
		// Key/value doesn't exist yet - user has no custom PATH set.
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "PATH") {
			continue
		}
		fields := strings.SplitN(line, "REG_", 2)
		if len(fields) != 2 {
			continue
		}
		// fields[1] looks like "EXPAND_SZ    <value>" or "SZ    <value>".
		rest := strings.TrimSpace(fields[1])
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// containsPathEntry reports whether dir already appears as an entry in
// the ';'-separated PATH string, ignoring a trailing backslash and case
// (Windows paths are case-insensitive).
func containsPathEntry(pathValue, dir string) bool {
	target := strings.ToLower(strings.TrimSuffix(dir, `\`))
	for _, entry := range strings.Split(pathValue, ";") {
		if strings.ToLower(strings.TrimSuffix(entry, `\`)) == target {
			return true
		}
	}
	return false
}

func copyFile(src, dst string, perm os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, perm)
}
