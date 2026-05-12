package deploy

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func DetectNativePkgs(projectPath string) []string {
	set := map[string]struct{}{}

	if fileExists(filepath.Join(projectPath, "package.json")) {
		for _, p := range detectNodeNativePkgs(projectPath) {
			set[p] = struct{}{}
		}
	}

	if hasPythonManifest(projectPath) {
		for _, p := range detectPythonNativePkgs(projectPath) {
			set[p] = struct{}{}
		}
	}

	// future: go.mod, Gemfile, Cargo.toml, etc.

	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ---------- Node (package.json) ----------

type pkgJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

var nodeNativeHints = map[string][]string{
	"sqlite3":        {"python3", "gcc", "gnumake", "pkg-config"},
	"better-sqlite3": {"python3", "gcc", "gnumake"},
	"bcrypt":         {"python3", "gcc", "gnumake"},
	"canvas":         {"python3", "gcc", "gnumake", "pkg-config", "cairo", "pango", "libjpeg"},
	"sharp":          {"vips", "pkg-config"},
	"node-sass":      {"python3", "gcc", "gnumake"},
	"grpc":           {"python3", "gcc", "gnumake"},
	"@grpc/grpc-js":  {"python3", "gcc", "gnumake"},
	"usb":            {"python3", "gcc", "gnumake", "libusb1"},
	"serialport":     {"python3", "gcc", "gnumake"},
	"node-pty":       {"python3", "gcc", "gnumake"},
	"re2":            {"python3", "gcc", "gnumake"},
	"puppeteer":      {"chromium"},
	"playwright":     {"chromium"},
}

var nodeBuildTriggerSubstrings = []string{
	"node-gyp",
	"rebuild",
	"build-from-source",
	"prebuild",
	"node-pre-gyp",
}

var nodeBaselineBuildPkgs = []string{"python3", "gcc", "gnumake", "pkg-config"}

func detectNodeNativePkgs(projectPath string) []string {
	data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
	if err != nil {
		return nil
	}

	var pj pkgJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return nil
	}

	set := map[string]struct{}{}

	allDeps := map[string]string{}

	maps.Copy(allDeps, pj.Dependencies)
	maps.Copy(allDeps, pj.DevDependencies)

	matched := false
	for name := range allDeps {
		if pkgs, ok := nodeNativeHints[name]; ok {
			matched = true
			for _, p := range pkgs {
				set[p] = struct{}{}
			}
		}
	}

	buildTriggered := false
	for _, hook := range []string{"preinstall", "install", "postinstall"} {
		if _, ok := pj.Scripts[hook]; ok {
			buildTriggered = true
			break
		}
	}
	if !buildTriggered {
		for _, script := range pj.Scripts {
			low := strings.ToLower(script)
			for _, sub := range nodeBuildTriggerSubstrings {
				if strings.Contains(low, sub) {
					buildTriggered = true
					break
				}
			}
			if buildTriggered {
				break
			}
		}
	}

	if buildTriggered && !matched {
		for _, p := range nodeBaselineBuildPkgs {
			set[p] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}

// ---------- Python (requirements.txt / pyproject.toml / Pipfile / setup.py) ----------

var pythonManifests = []string{
	"requirements.txt",
	"pyproject.toml",
	"Pipfile",
	"setup.py",
	"setup.cfg",
}

var pythonNativeHints = map[string][]string{
	"psycopg2":     {"postgresql", "gcc", "pkg-config"},
	"psycopg":      {"postgresql", "gcc", "pkg-config"},
	"mysqlclient":  {"libmysqlclient", "gcc", "pkg-config"},
	"pillow":       {"zlib", "libjpeg", "libtiff", "freetype", "gcc"},
	"lxml":         {"libxml2", "libxslt", "gcc", "pkg-config"},
	"cryptography": {"openssl", "libffi", "gcc", "pkg-config"},
	"pycurl":       {"curl", "openssl", "gcc", "pkg-config"},
	"numpy":        {"gcc", "gfortran", "blas", "lapack"},
	"scipy":        {"gcc", "gfortran", "blas", "lapack"},
	"pandas":       {"gcc"},
	"matplotlib":   {"freetype", "libpng", "gcc", "pkg-config"},
	"uwsgi":        {"gcc", "pcre"},
	"pyodbc":       {"unixODBC", "gcc"},
	"pyzmq":        {"zeromq", "gcc", "pkg-config"},
	"pycairo":      {"cairo", "gcc", "pkg-config"},
	"pygobject":    {"glib", "gobject-introspection", "gcc", "pkg-config"},
	"pyaudio":      {"portaudio", "gcc"},
	"pynacl":       {"libsodium", "gcc"},
	"bcrypt":       {"libffi", "gcc"},
	"argon2-cffi":  {"libffi", "gcc"},
	"gevent":       {"gcc", "libev"},
	"greenlet":     {"gcc"},
	"grpcio":       {"gcc", "openssl"},
	"dbus-python":  {"dbus", "glib", "gcc", "pkg-config"},
	"python-ldap":  {"openldap", "cyrus_sasl", "gcc", "pkg-config"},
	"pysam":        {"zlib", "bzip2", "xz", "gcc"},
	"shapely":      {"geos", "gcc"},
	"cartopy":      {"geos", "proj", "gcc"},
	"rasterio":     {"gdal", "gcc"},
	"fiona":        {"gdal", "gcc"},
	"h5py":         {"hdf5", "gcc", "pkg-config"},
	"netcdf4":      {"netcdf", "hdf5", "gcc", "pkg-config"},
}

var pythonBaselineBuildPkgs = []string{"gcc", "gnumake", "pkg-config", "libffi", "openssl"}

var pyReqLineRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.\-]*`)

func hasPythonManifest(projectPath string) bool {
	for _, m := range pythonManifests {
		if fileExists(filepath.Join(projectPath, m)) {
			return true
		}
	}
	return false
}

func detectPythonNativePkgs(projectPath string) []string {
	set := map[string]struct{}{}
	matched := false

	depNames := collectPythonDepNames(projectPath)
	for _, name := range depNames {
		key := strings.ToLower(name)
		// psycopg2-binary etc. ship wheels with system libs bundled — skip
		if strings.HasSuffix(key, "-binary") {
			continue
		}
		if pkgs, ok := pythonNativeHints[key]; ok {
			matched = true
			for _, p := range pkgs {
				set[p] = struct{}{}
			}
		}
	}

	// C-extension build likely if setup.py or pyproject build-backend implies compile
	buildTriggered := fileExists(filepath.Join(projectPath, "setup.py"))
	if !buildTriggered {
		if data, err := os.ReadFile(filepath.Join(projectPath, "pyproject.toml")); err == nil {
			low := strings.ToLower(string(data))
			if strings.Contains(low, "cython") || strings.Contains(low, "cffi") ||
				strings.Contains(low, "scikit-build") || strings.Contains(low, "meson-python") ||
				strings.Contains(low, "maturin") {
				buildTriggered = true
			}
		}
	}

	if buildTriggered && !matched {
		for _, p := range pythonBaselineBuildPkgs {
			set[p] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}

func collectPythonDepNames(projectPath string) []string {
	var names []string

	if data, err := os.ReadFile(filepath.Join(projectPath, "requirements.txt")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
				continue
			}
			if m := pyReqLineRe.FindString(line); m != "" {
				names = append(names, m)
			}
		}
	}

	// crude substring scan for pyproject.toml / Pipfile — avoids pulling TOML parser
	for _, f := range []string{"pyproject.toml", "Pipfile", "setup.py", "setup.cfg"} {
		data, err := os.ReadFile(filepath.Join(projectPath, f))
		if err != nil {
			continue
		}
		low := strings.ToLower(string(data))
		for hint := range pythonNativeHints {
			if strings.Contains(low, hint) {
				names = append(names, hint)
			}
		}
	}

	return names
}
