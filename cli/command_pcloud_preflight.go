package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	pcloud "github.com/kopia-backends/go-pcloud"
)

type commandPCloudPreflight struct {
	svc           appServices
	accessToken   string
	authToken     string
	apiHost       string
	timeout       time.Duration
	requiredRoots []string
	targets       []string
}

func (c *commandPCloudPreflight) setup(svc appServices, parent commandParent) {
	c.svc = svc

	cmd := parent.Command("preflight", "Read-only pCloud connectivity and folder checks before a production operation.")
	cmd.Flag("access-token", "pCloud OAuth access token").Envar(svc.EnvName("PCLOUD_ACCESS_TOKEN")).StringVar(&c.accessToken)
	cmd.Flag("auth-token", "pCloud legacy auth token").Hidden().Envar(svc.EnvName("PCLOUD_AUTH_TOKEN")).StringVar(&c.authToken)
	cmd.Flag("api-host", "pCloud API endpoint").Envar(svc.EnvName("PCLOUD_API_HOST")).StringVar(&c.apiHost)
	cmd.Flag("required-root", "pCloud folder that must exist, repeatable or comma-separated").StringsVar(&c.requiredRoots)
	cmd.Flag("target", "Optional pCloud folder to inspect, repeatable or comma-separated").StringsVar(&c.targets)
	cmd.Flag("timeout", "Overall preflight timeout").Default("2m").DurationVar(&c.timeout)
	cmd.Action(svc.noRepositoryAction(c.run))
}

func (c *commandPCloudPreflight) run(ctx context.Context) error {
	if c.accessToken == "" && c.authToken == "" {
		return fmt.Errorf("missing --access-token, --auth-token, PCLOUD_ACCESS_TOKEN or PCLOUD_AUTH_TOKEN")
	}

	requiredRoots := splitPCloudList(c.requiredRoots)
	targets := splitPCloudList(c.targets)
	if len(requiredRoots) == 0 {
		return fmt.Errorf("at least one --required-root is required")
	}

	client := pcloud.NewClient(pcloud.Options{
		AccessToken: c.accessToken,
		AuthToken:   c.authToken,
		APIHost:     c.apiHost,
		Timeout:     30 * time.Second, //nolint:mnd
		MaxRetries:  3,                //nolint:mnd
	})

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	apiHostDisplay := c.apiHost
	if apiHostDisplay == "" {
		apiHostDisplay = pcloud.DefaultAPIHost
	}

	return runPCloudPreflight(ctx, client, c.svc.stdout(), apiHostDisplay, requiredRoots, targets)
}

// runPCloudPreflight and its helpers below take an io.Writer instead of
// writing to stdout directly so they can be unit-tested without going
// through kingpin/appServices, mirroring the pre-migration tool at
// kopia-pcloud-infra/tools/pcloud-preflight/main.go.
func runPCloudPreflight(ctx context.Context, client *pcloud.Client, w io.Writer, apiHostDisplay string, requiredRoots, targets []string) error {
	fmt.Fprintln(w, "pCloud production preflight")  //nolint:errcheck
	fmt.Fprintf(w, "api_host=%s\n", apiHostDisplay) //nolint:errcheck

	for _, root := range requiredRoots {
		if err := inspectPCloudFolder(ctx, client, w, "required", root, true); err != nil {
			return err
		}
	}
	for _, target := range targets {
		if err := inspectPCloudFolder(ctx, client, w, "target", target, false); err != nil {
			return err
		}
	}

	fmt.Fprintln(w, "Preflight read-only checks passed.") //nolint:errcheck
	return nil
}

func inspectPCloudFolder(ctx context.Context, client *pcloud.Client, w io.Writer, label, itemPath string, required bool) error {
	cleaned, err := cleanSafePCloudPath(itemPath)
	if err != nil {
		return err
	}

	entries, err := client.ListFolder(ctx, cleaned)
	if err != nil {
		if isMissingPCloudPath(err) {
			if required {
				return fmt.Errorf("%s folder missing: %s", label, cleaned)
			}
			fmt.Fprintf(w, "%s folder missing: %s\n", label, cleaned) //nolint:errcheck
			return nil
		}
		return fmt.Errorf("list %s folder %s: %w", label, cleaned, err)
	}

	files, folders, size := summarizePCloudEntries(entries)
	fmt.Fprintf(w, "%s folder ok: %s files=%d folders=%d bytes=%d\n", label, cleaned, files, folders, size) //nolint:errcheck
	return nil
}

func summarizePCloudEntries(entries []pcloud.Entry) (files, folders int, size int64) {
	for _, entry := range entries {
		switch {
		case entry.File != nil:
			files++
			size += entry.File.Size
		case entry.Folder != nil:
			folders++
		}
	}
	return files, folders, size
}

// splitPCloudList flattens repeatable --flag values that may also contain
// comma-separated items, e.g. `--target a --target b,c` -> ["a", "b", "c"].
func splitPCloudList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, item := range strings.Split(v, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
	}
	return out
}

func cleanSafePCloudPath(v string) (string, error) {
	cleaned := cleanPCloudPath(v)
	switch cleaned {
	case "", "/", ".":
		return "", fmt.Errorf("refusing unsafe pCloud path=%q", v)
	}
	return cleaned, nil
}

func cleanPCloudPath(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	cleaned := path.Clean(v)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func isMissingPCloudPath(err error) bool {
	return errors.Is(err, pcloud.ErrNotFound) || errors.Is(err, pcloud.ErrParentNotFound)
}
