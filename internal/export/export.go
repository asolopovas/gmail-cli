package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andrius/gmail-cli/internal/email"
)

type Writer struct {
	Root string
}

type Result struct {
	Directory   string
	EmailFile   string
	Attachments []string
}

func (w Writer) Save(message email.Message) (Result, error) {
	root := w.Root
	if root == "" {
		root = "exports"
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve export root: %w", err)
	}
	if err := os.MkdirAll(rootAbs, 0o700); err != nil {
		return Result{}, fmt.Errorf("create export root %q: %w", rootAbs, err)
	}

	dirName := folderName(message)
	dir, err := uniquePath(rootAbs, dirName)
	if err != nil {
		return Result{}, err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create message export dir %q: %w", dir, err)
	}

	emailFile := filepath.Join(dir, "email.txt")
	if err := os.WriteFile(emailFile, []byte(email.Render(message)), 0o600); err != nil {
		return Result{}, fmt.Errorf("write email text %q: %w", emailFile, err)
	}

	result := Result{Directory: dir, EmailFile: emailFile}
	used := map[string]struct{}{"email.txt": {}}
	for i, att := range message.Attachments {
		name := email.SafeFilename(att.Filename, fmt.Sprintf("attachment-%02d", i+1), 160)
		name = uniqueName(name, used)
		used[name] = struct{}{}
		path := filepath.Join(dir, name)
		if !isWithin(dir, path) {
			return Result{}, fmt.Errorf("attachment path escaped export dir: %q", name)
		}
		if err := os.WriteFile(path, att.Data, 0o600); err != nil {
			return Result{}, fmt.Errorf("write attachment %q: %w", path, err)
		}
		result.Attachments = append(result.Attachments, path)
	}
	return result, nil
}

func folderName(message email.Message) string {
	timestamp := "unknown-date"
	if !message.InternalAt.IsZero() {
		timestamp = message.InternalAt.Local().Format("06-01-02 150405")
	}
	desc := strings.TrimSpace(message.Subject)
	if desc == "" {
		desc = message.From
	}
	if desc == "" {
		desc = message.ID
	}
	return email.SafeLabel(timestamp+" - "+desc, "email", 180)
}

func uniquePath(root string, name string) (string, error) {
	for i := range 10000 {
		candidateName := name
		if i > 0 {
			candidateName = fmt.Sprintf("%s-%03d", name, i)
		}
		candidate := filepath.Join(root, candidateName)
		if !isWithin(root, candidate) {
			return "", fmt.Errorf("export path escaped root: %q", candidateName)
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("stat export path %q: %w", candidate, err)
		}
	}
	return "", fmt.Errorf("too many duplicate export directories for %q", name)
}

func uniqueName(name string, used map[string]struct{}) string {
	if _, ok := used[name]; !ok {
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%03d%s", stem, i, ext)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func isWithin(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
