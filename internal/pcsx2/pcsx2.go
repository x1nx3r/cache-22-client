package pcsx2

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func AppImageURL(version string) string {
	return fmt.Sprintf("https://github.com/PCSX2/pcsx2/releases/download/%s/pcsx2-%s-linux-appimage-x64-Qt.AppImage", version, version)
}

func AppImagePath(dir, version string) string {
	return filepath.Join(dir, "pcsx2-"+version+".AppImage")
}

func Ensure(dir, version string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := AppImagePath(dir, version)
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		return path, nil
	}
	res, err := http.Get(AppImageURL(version))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download pcsx2 %s: %s", version, res.Status)
	}
	tmp := path + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	f.Close()
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return path, os.Rename(tmp, path)
}

func DataRoot(dataDir string) string {
	return filepath.Join(dataDir, "PCSX2")
}

func staleRoot(dataDir string) string {
	return filepath.Join(dataDir, "pcsx2")
}

func BiosDir(dataDir string) string {
	return filepath.Join(DataRoot(dataDir), "bios")
}

func LegacyBiosDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "PCSX2", "bios")
}

func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}

func HasBIOS(dataDir string) bool {
	return dirHasFiles(BiosDir(dataDir)) || dirHasFiles(LegacyBiosDir())
}

func InstallBIOS(dataDir string, srcs []string) (int, error) {
	dir := BiosDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	done := 0
	for _, src := range srcs {
		st, err := os.Stat(src)
		if err != nil || st.IsDir() {
			continue
		}
		in, err := os.Open(src)
		if err != nil {
			continue
		}
		out, err := os.OpenFile(filepath.Join(dir, filepath.Base(src)), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			in.Close()
			continue
		}
		_, err = io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			continue
		}
		done++
	}
	return done, nil
}

func MigrateLegacyBIOS(dataDir string) (int, error) {
	if dirHasFiles(BiosDir(dataDir)) {
		return 0, nil
	}
	for _, dir := range []string{filepath.Join(staleRoot(dataDir), "bios"), LegacyBiosDir()} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var srcs []string
		for _, e := range entries {
			if !e.IsDir() {
				srcs = append(srcs, filepath.Join(dir, e.Name()))
			}
		}
		if len(srcs) == 0 {
			continue
		}
		n, err := InstallBIOS(dataDir, srcs)
		if err != nil {
			return n, err
		}
		if dir == filepath.Join(staleRoot(dataDir), "bios") {
			os.RemoveAll(staleRoot(dataDir))
		}
		return n, nil
	}
	return 0, nil
}

func Prepare(dataDir string) error {
	root := DataRoot(dataDir)
	if err := os.MkdirAll(filepath.Join(root, "inis"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(BiosDir(dataDir), 0o755); err != nil {
		return err
	}
	if _, err := MigrateLegacyBIOS(dataDir); err != nil {
		return err
	}
	return ensureINI(filepath.Join(root, "inis", "PCSX2.ini"), BiosDir(dataDir))
}

func ensureINI(path, biosDir string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		seeded := strings.Replace(defaultINI, "__CACHE22_BIOS_DIR__", biosDir, 1)
		return os.WriteFile(path, []byte(seeded), 0o644)
	}
	var out []string
	section := ""
	foundWizard, foundBios := false, false
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
		} else if section == "[UI]" && strings.HasPrefix(trimmed, "SetupWizardIncomplete") {
			line = "SetupWizardIncomplete = false"
			foundWizard = true
		} else if section == "[Folders]" && strings.HasPrefix(trimmed, "Bios") {
			line = "Bios = " + biosDir
			foundBios = true
		}
		out = append(out, line)
	}
	if !foundWizard {
		out = append(out, "[UI]", "SetupWizardIncomplete = false")
	}
	if !foundBios {
		out = append(out, "[Folders]", "Bios = "+biosDir)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

func LaunchArgs(dataDir, iso string) []string {
	return []string{"-batch", "-fastboot", "-fullscreen", "-datapath", dataDir, iso}
}

func Start(appImage, dataDir, iso string) (*exec.Cmd, error) {
	args := LaunchArgs(dataDir, iso)
	cmd := exec.Command(appImage, args...)
	env := append(os.Environ(), "APPIMAGELAUNCHER_DISABLE=1")
	if needsExtractRun() {
		env = append(env, "APPIMAGE_EXTRACT_AND_RUN=1")
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func Launch(appImage, dataDir, iso string) error {
	cmd, err := Start(appImage, dataDir, iso)
	if err != nil {
		return err
	}
	return cmd.Wait()
}

func needsExtractRun() bool {
	out, err := exec.Command("ldconfig", "-p").Output()
	if err != nil {
		return true
	}
	return !strings.Contains(string(out), "libfuse.so.2")
}
