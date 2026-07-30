package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pcloud "github.com/kopia-backends/go-pcloud"
)

type commandPCloudMkdir struct {
	svc         appServices
	accessToken string
	authToken   string
	apiHost     string
	path        string
	confirm     string
	timeout     time.Duration
}

func (c *commandPCloudMkdir) setup(svc appServices, parent commandParent) {
	c.svc = svc

	cmd := parent.Command("mkdir", "Create a pCloud folder (and any missing parents) after explicit confirmation.")
	cmd.Flag("access-token", "pCloud OAuth access token").Envar(svc.EnvName("PCLOUD_ACCESS_TOKEN")).StringVar(&c.accessToken)
	cmd.Flag("auth-token", "pCloud legacy auth token").Hidden().Envar(svc.EnvName("PCLOUD_AUTH_TOKEN")).StringVar(&c.authToken)
	cmd.Flag("api-host", "pCloud API endpoint").Envar(svc.EnvName("PCLOUD_API_HOST")).StringVar(&c.apiHost)
	cmd.Flag("path", "pCloud folder path to create").Required().StringVar(&c.path)
	cmd.Flag("confirm", "Must exactly match --path after normalization").Required().StringVar(&c.confirm)
	cmd.Flag("timeout", "pCloud mkdir timeout").Default("2m").DurationVar(&c.timeout)
	cmd.Action(svc.noRepositoryAction(c.run))
}

func (c *commandPCloudMkdir) run(ctx context.Context) error {
	if c.accessToken == "" && c.authToken == "" {
		return fmt.Errorf("missing --access-token, --auth-token, PCLOUD_ACCESS_TOKEN or PCLOUD_AUTH_TOKEN")
	}

	// Path and confirmation are validated before any network activity, same
	// as the pre-migration tool at kopia-pcloud-infra/tools/pcloud-mkdir.
	cleaned, err := cleanSafePCloudPath(c.path)
	if err != nil {
		return err
	}
	cleanConfirm, err := cleanSafePCloudPath(c.confirm)
	if err != nil {
		return fmt.Errorf("invalid --confirm path: %w", err)
	}
	if cleanConfirm != cleaned {
		return fmt.Errorf("refusing mkdir unless --confirm=%s", cleaned)
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

	if err := ensurePCloudFolder(ctx, client, cleaned); err != nil {
		return err
	}
	fmt.Fprintf(c.svc.stdout(), "folder ok: %s\n", cleaned) //nolint:errcheck
	return nil
}

func ensurePCloudFolder(ctx context.Context, client *pcloud.Client, folderPath string) error {
	if _, err := client.ListFolder(ctx, folderPath); err == nil {
		return nil
	} else if !isMissingPCloudPath(err) {
		return fmt.Errorf("list %s: %w", folderPath, err)
	}

	current := ""
	for _, segment := range strings.Split(strings.Trim(folderPath, "/"), "/") {
		if segment == "" {
			continue
		}
		current += "/" + segment
		if _, err := client.CreateFolder(ctx, current); err != nil && !errors.Is(err, pcloud.ErrAlreadyExists) {
			return fmt.Errorf("create %s: %w", current, err)
		}
	}

	if _, err := client.ListFolder(ctx, folderPath); err != nil {
		return fmt.Errorf("verify %s: %w", folderPath, err)
	}
	return nil
}
