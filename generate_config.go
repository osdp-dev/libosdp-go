//go:build ignore

// Command generate_config vendors the libosdp C sources from the libosdp/ git
// submodule into this package so it builds as a standalone, `go get`-able module
// (cgo only compiles .c files that live in the package directory), and generates
// osdp_config.h from the upstream template with the submodule's version/git info
// baked in.
//
// Run it with:
//
//	go generate .
//
// after initialising the submodule (git submodule update --init --recursive).
// The resulting vendored tree (flattened *.c files plus the clib/ header tree)
// is committed so consumers need only `go get` — no submodule, CMake, or git at
// build time. Re-run after bumping the submodule and commit the result.
package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// cSources are compiled by cgo and therefore copied (flattened) into the
// package directory. This mirrors the set the Python wheel vendors, with
// OPT_OSDP_PACKET_TRACE enabled (hence osdp_diag.c + pcap_gen.c), plus
// osdp_trs.c for OPT_BUILD_OSDP_TRS (set in internal/clib/clib.go).
var cSources = []string{
	"src/osdp_common.c",
	"src/osdp_phy.c",
	"src/osdp_sc.c",
	"src/osdp_file.c",
	"src/osdp_multipart.c",
	"src/osdp_pd.c",
	"src/osdp_cp.c",
	"src/osdp_metrics.c",
	"src/osdp_diag.c",
	"src/osdp_bio.c",
	"src/osdp_piv.c",
	"src/osdp_trs.c",
	"src/crypto/tinyaes_src.c",
	"src/crypto/tinyaes.c",
	"utils/src/list.c",
	"utils/src/queue.c",
	"utils/src/utils.c",
	"utils/src/logger.c",
	"utils/src/disjoint_set.c",
	"utils/src/crc16.c",
	"utils/src/pcap_gen.c",
}

// headerDirs maps a repo-relative source directory of headers to the
// destination directory (relative to the package) they are copied into. All
// *.h in the source dir are copied; unused headers are harmless.
var headerDirs = []struct{ src, dst string }{
	{"include", "internal/clib/include"},
	{"src", "internal/clib/src"},
	{"src/crypto", "internal/clib/src/crypto"},
	{"utils/include/utils", "internal/clib/utils/include/utils"},
}

const (
	projectName = "libosdp"
	// clibDir holds the vendored C compiled by the internal/clib cgo package,
	// keeping the C out of the public binding package directory.
	clibDir = "internal/clib"
)

func main() {
	pkgDir, err := os.Getwd() // `go generate` runs in the package directory
	must(err)
	// The libosdp C sources live in the git submodule at <pkg>/libosdp.
	repoRoot := filepath.Join(pkgDir, "libosdp")
	if _, err := os.Stat(filepath.Join(repoRoot, "src", "osdp_common.c")); err != nil {
		must(fmt.Errorf("libosdp submodule not initialised at %s (run: git submodule update --init --recursive)", repoRoot))
	}

	// Remove the previously generated content so stale files never linger, while
	// preserving the hand-written clib.go package file. Only the vendored .c and
	// the header subdirectories are generated.
	for _, sub := range []string{"include", "src", "utils"} {
		must(os.RemoveAll(filepath.Join(pkgDir, clibDir, sub)))
	}
	for _, rel := range cSources {
		_ = os.Remove(filepath.Join(pkgDir, clibDir, filepath.Base(rel)))
	}
	// Also clear any leftover flat layout from before the internal/clib split.
	must(os.RemoveAll(filepath.Join(pkgDir, "clib")))
	for _, rel := range cSources {
		_ = os.Remove(filepath.Join(pkgDir, filepath.Base(rel)))
	}

	// Flatten the C sources into the internal/clib package directory (cgo only
	// compiles .c files that sit beside the package's Go file, clib.go).
	for _, rel := range cSources {
		copyFile(filepath.Join(repoRoot, rel), filepath.Join(pkgDir, clibDir, filepath.Base(rel)))
	}

	// Copy the header tree (preserving the include layout under internal/clib/).
	for _, hd := range headerDirs {
		copyHeaders(filepath.Join(repoRoot, hd.src), filepath.Join(pkgDir, hd.dst))
	}

	// Generate osdp_config.h from the upstream template.
	genConfig(
		filepath.Join(repoRoot, "src", "osdp_config.h.in"),
		filepath.Join(pkgDir, clibDir, "src", "osdp_config.h"),
		repoRoot,
	)

	writeTreeStamp(filepath.Join(pkgDir, clibDir))

	fmt.Println("vendored libosdp C sources from submodule (run: go build .)")
}

// writeTreeStamp records a hash of the vendored C tree in a Go file of the
// clib package. The go build cache keys cgo objects on the package's own
// files and does not see headers in its subdirectories, so without this a
// regenerated tree can silently link objects built against the old headers.
// osdp_config.h is left out: it carries host-specific git metadata and CI
// excludes it from its staleness check too.
func writeTreeStamp(dir string) {
	var files []string
	must(filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		ext := filepath.Ext(path)
		if !d.IsDir() && (ext == ".c" || ext == ".h") && rel != filepath.Join("src", "osdp_config.h") {
			files = append(files, rel)
		}
		return nil
	}))
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		must(err)
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(data))
		h.Write(data)
	}
	out := fmt.Sprintf(`// Code generated by "go generate" in the parent package; DO NOT EDIT.

package clib

// vendoredTreeHash changes whenever the vendored C sources or headers do, so
// the go build cache recompiles them; see writeTreeStamp in generate_config.go.
const vendoredTreeHash = "%x"
`, h.Sum(nil))
	must(os.WriteFile(filepath.Join(dir, "zz_tree_stamp.go"), []byte(out), 0o644))
}

func genConfig(tmpl, dst, repoRoot string) {
	raw, err := os.ReadFile(tmpl)
	must(err)

	version, prerelease := cmakeProjectVersion(repoRoot)
	rev := git(repoRoot, "describe", "--tags", "--long", "--always", "--abbrev=7")
	tag := git(repoRoot, "describe", "--exact-match", "--tags", "HEAD")
	dirty := gitDiffMarker(repoRoot)
	if tag != "" {
		tag += dirty
	}
	branch := git(repoRoot, "symbolic-ref", "--short", "-q", "HEAD")
	if branch == "" {
		branch = "detached"
	}

	repl := map[string]string{
		"PROJECT_VERSION":     version,
		"LIBOSDP_VERSION_STR": versionString(version, prerelease, rev, tag),
		"PROJECT_NAME":        projectName,
		"GIT_BRANCH":          branch,
		"GIT_REV":             rev,
		"GIT_TAG":             tag,
		"GIT_DIFF":            dirty,
		"REPO_ROOT":           repoRoot,
	}

	out := regexp.MustCompile(`@(\w+)@`).ReplaceAllStringFunc(string(raw), func(m string) string {
		key := strings.Trim(m, "@")
		if v, ok := repl[key]; ok {
			return v
		}
		return m
	})
	must(os.MkdirAll(filepath.Dir(dst), 0o755))
	must(os.WriteFile(dst, []byte(out), 0o644))
}

// projectVersion derives the X.Y.Z version from the latest git tag, falling
// back to the version compiled into the build if git is unavailable.
// cmakeProjectVersion reads the numeric version from upstream's project() call
// and the LIBOSDP_PRERELEASE marker ("" once released) from CMakeLists.txt.
func cmakeProjectVersion(repoRoot string) (version, prerelease string) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "CMakeLists.txt"))
	must(err)
	m := regexp.MustCompile(`project\(libosdp VERSION ([0-9.]+)\)`).FindSubmatch(raw)
	if m == nil {
		must(fmt.Errorf("no project(libosdp VERSION ...) in CMakeLists.txt"))
	}
	version = string(m[1])
	if p := regexp.MustCompile(`set\(LIBOSDP_PRERELEASE "([^"]*)"\)`).FindSubmatch(raw); p != nil {
		prerelease = string(p[1])
	}
	return version, prerelease
}

// versionString mirrors LIBOSDP_VERSION_STR in upstream's cmake/GitInfo.cmake:
// a pre-release, or any build not sitting on a release tag, carries its git
// position so it cannot be mistaken for the release.
func versionString(version, prerelease, rev, tag string) string {
	pos := regexp.MustCompile(`-([0-9]+)-g([0-9a-f]+)$`).FindStringSubmatch(rev)
	if prerelease == "" {
		if tag != "" {
			return version
		}
		if pos != nil {
			return version + "+" + pos[1] + ".g" + pos[2]
		}
		if rev != "" {
			return version + "+g" + rev
		}
		return version
	}
	s := version + "-" + prerelease
	if pos != nil {
		return s + "." + pos[1] + "+g" + pos[2]
	}
	if rev != "" {
		return s + "+g" + rev
	}
	return s
}

func gitDiffMarker(repoRoot string) string {
	if git(repoRoot, "status", "--porcelain", "--untracked-files=normal") != "" {
		return "+"
	}
	return ""
}

func git(repoRoot string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func copyHeaders(srcDir, dstDir string) {
	entries, err := os.ReadDir(srcDir)
	must(err)
	must(os.MkdirAll(dstDir, 0o755))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
			continue
		}
		copyFile(filepath.Join(srcDir, e.Name()), filepath.Join(dstDir, e.Name()))
	}
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	must(err)
	must(os.MkdirAll(filepath.Dir(dst), 0o755))
	must(os.WriteFile(dst, data, 0o644))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate_config:", err)
		os.Exit(1)
	}
}
