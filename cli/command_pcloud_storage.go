package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pcloud "github.com/kopia-backends/go-pcloud"
	"github.com/kopia/kopia/internal/units"
)

type commandPCloudStorage struct {
	svc         appServices
	accessToken string
	authToken   string
	apiHost     string
	rootPath    string
	watch       bool
	jsonOutput  bool
	interval    time.Duration
	timeout     time.Duration
}

type pcloudStorageReport struct {
	Timestamp             time.Time `json:"timestamp"`
	AccountCapacityBytes  uint64    `json:"accountCapacityBytes"`
	AccountUsedBytes      uint64    `json:"accountUsedBytes"`
	AccountAvailableBytes uint64    `json:"accountAvailableBytes"`
	TargetPath            string    `json:"targetPath,omitempty"`
	TargetOwnership       string    `json:"targetOwnership,omitempty"`
	TargetCapacityKnown   bool      `json:"targetCapacityKnown"`
}

func (c *commandPCloudStorage) setup(svc appServices, parent commandParent) {
	c.svc = svc

	cmd := parent.Command("storage", "Display or monitor pCloud storage capacity.")
	cmd.Flag("access-token", "pCloud OAuth access token").Envar(svc.EnvName("PCLOUD_ACCESS_TOKEN")).StringVar(&c.accessToken)
	cmd.Flag("auth-token", "pCloud legacy auth token").Hidden().Envar(svc.EnvName("PCLOUD_AUTH_TOKEN")).StringVar(&c.authToken)
	cmd.Flag("api-host", "pCloud API endpoint").Default("https://eapi.pcloud.com").Envar(svc.EnvName("PCLOUD_API_HOST")).StringVar(&c.apiHost)
	cmd.Flag("root-path", "Optional pCloud target used to determine whether account quota applies").StringVar(&c.rootPath)
	cmd.Flag("watch", "Continuously monitor storage capacity").BoolVar(&c.watch)
	cmd.Flag("interval", "Polling interval used with --watch").Default("60s").DurationVar(&c.interval)
	cmd.Flag("json", "Emit one JSON object per measurement").BoolVar(&c.jsonOutput)
	cmd.Flag("timeout", "pCloud request timeout").Default("30s").DurationVar(&c.timeout)
	cmd.Action(svc.noRepositoryAction(c.run))
}

func (c *commandPCloudStorage) run(ctx context.Context) error {
	if c.accessToken == "" && c.authToken == "" {
		return fmt.Errorf("missing --access-token, --auth-token, PCLOUD_ACCESS_TOKEN or PCLOUD_AUTH_TOKEN")
	}
	if c.watch && c.interval <= 0 {
		return fmt.Errorf("--interval must be greater than zero")
	}

	client := pcloud.NewClient(pcloud.Options{
		AccessToken: c.accessToken,
		AuthToken:   c.authToken,
		APIHost:     c.apiHost,
		Timeout:     c.timeout,
	})

	for {
		report, err := c.measure(ctx, client)
		if err != nil {
			return err
		}
		if err := c.printReport(report); err != nil {
			return err
		}
		if !c.watch {
			return nil
		}

		timer := time.NewTimer(c.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (c *commandPCloudStorage) measure(ctx context.Context, client *pcloud.Client) (pcloudStorageReport, error) {
	usage, err := client.GetStorageUsage(ctx)
	if err != nil {
		return pcloudStorageReport{}, err
	}

	report := pcloudStorageReport{
		Timestamp:             time.Now().UTC(),
		AccountCapacityBytes:  usage.QuotaBytes,
		AccountUsedBytes:      usage.UsedBytes,
		AccountAvailableBytes: usage.FreeBytes(),
		TargetPath:            c.rootPath,
		TargetCapacityKnown:   c.rootPath == "",
	}
	if c.rootPath == "" {
		return report, nil
	}

	entry, err := client.Stat(ctx, c.rootPath)
	if err != nil {
		if errors.Is(err, pcloud.ErrNotFound) {
			report.TargetOwnership = "missing"
			return report, nil
		}
		return pcloudStorageReport{}, fmt.Errorf("stat pCloud target %q: %w", c.rootPath, err)
	}
	if entry.Folder == nil {
		return pcloudStorageReport{}, fmt.Errorf("pCloud target %q is not a folder", c.rootPath)
	}

	switch {
	case !entry.Folder.OwnershipKnown:
		report.TargetOwnership = "unknown"
	case entry.Folder.IsMine:
		report.TargetOwnership = "owned"
		report.TargetCapacityKnown = true
	default:
		report.TargetOwnership = "shared"
	}
	return report, nil
}

func (c *commandPCloudStorage) printReport(report pcloudStorageReport) error {
	if c.jsonOutput {
		data, err := json.Marshal(report)
		if err != nil {
			return err
		}
		fmt.Fprintln(c.svc.stdout(), string(data)) //nolint:errcheck
		return nil
	}

	fmt.Fprintf(c.svc.stdout(), "pCloud storage at %s\n", report.Timestamp.Format(time.RFC3339))                                                     //nolint:errcheck
	fmt.Fprintf(c.svc.stdout(), "Account capacity:  %s (%d bytes)\n", units.BytesString(report.AccountCapacityBytes), report.AccountCapacityBytes)   //nolint:errcheck
	fmt.Fprintf(c.svc.stdout(), "Account used:      %s (%d bytes)\n", units.BytesString(report.AccountUsedBytes), report.AccountUsedBytes)           //nolint:errcheck
	fmt.Fprintf(c.svc.stdout(), "Account available: %s (%d bytes)\n", units.BytesString(report.AccountAvailableBytes), report.AccountAvailableBytes) //nolint:errcheck
	if report.TargetPath != "" {
		fmt.Fprintf(c.svc.stdout(), "Target:            %s\n", report.TargetPath)      //nolint:errcheck
		fmt.Fprintf(c.svc.stdout(), "Target ownership:  %s\n", report.TargetOwnership) //nolint:errcheck
		switch {
		case report.TargetCapacityKnown:
			fmt.Fprintln(c.svc.stdout(), "Target capacity:   account quota applies") //nolint:errcheck
		case report.TargetOwnership == "shared":
			fmt.Fprintln(c.svc.stdout(), "Target capacity:   unknown (shared-folder owner quota is not exposed to this account)") //nolint:errcheck
		case report.TargetOwnership == "missing":
			fmt.Fprintln(c.svc.stdout(), "Target capacity:   unknown (target folder does not exist yet)") //nolint:errcheck
		default:
			fmt.Fprintln(c.svc.stdout(), "Target capacity:   unknown (folder ownership was not returned by pCloud)") //nolint:errcheck
		}
	}
	return nil
}
