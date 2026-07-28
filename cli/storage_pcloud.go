package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"

	pcloud "github.com/kopia-backends/go-pcloud"
	"github.com/kopia-backends/kopia-pcloud/pcloudblob"
	"github.com/kopia/kopia/repo/blob"
)

type storagePCloudFlags struct {
	options       pcloudblob.Options
	clientID      string
	clientSecret  string
	oauthCode     string
	callbackURL   string
	oauthHostname string
	timeout       time.Duration
}

func (c *storagePCloudFlags) Setup(svc StorageProviderServices, cmd *kingpin.CmdClause) {
	cmd.Flag("access-token", "pCloud OAuth access token").Envar(svc.EnvName("PCLOUD_ACCESS_TOKEN")).StringVar(&c.options.AccessToken)
	cmd.Flag("auth-token", "pCloud legacy auth token").Hidden().Envar(svc.EnvName("PCLOUD_AUTH_TOKEN")).StringVar(&c.options.AuthToken)
	cmd.Flag("client-id", "pCloud OAuth client id used with --oauth-code or --callback-url").Envar(svc.EnvName("PCLOUD_CLIENT_ID")).StringVar(&c.clientID)
	cmd.Flag("client-secret", "pCloud OAuth client secret used with --oauth-code or --callback-url").Envar(svc.EnvName("PCLOUD_CLIENT_SECRET")).StringVar(&c.clientSecret)
	cmd.Flag("oauth-code", "pCloud OAuth authorization code to exchange for an access token").Envar(svc.EnvName("PCLOUD_OAUTH_CODE")).StringVar(&c.oauthCode)
	cmd.Flag("callback-url", "Full pCloud OAuth callback URL to parse and exchange").Envar(svc.EnvName("PCLOUD_OAUTH_CALLBACK_URL")).StringVar(&c.callbackURL)
	cmd.Flag("oauth-hostname", "pCloud OAuth hostname displayed with --oauth-code").Envar(svc.EnvName("PCLOUD_OAUTH_HOSTNAME")).StringVar(&c.oauthHostname)
	cmd.Flag("root-path", "Root folder path in pCloud for the Kopia repository").Required().Envar(svc.EnvName("PCLOUD_ROOT_PATH")).StringVar(&c.options.RootPath)
	cmd.Flag("api-host", "pCloud API endpoint").Default("https://eapi.pcloud.com").Envar(svc.EnvName("PCLOUD_API_HOST")).StringVar(&c.options.APIHost)
	cmd.Flag("download-host", "Optional pCloud download host").Hidden().StringVar(&c.options.DownloadHost)
	cmd.Flag("timeout", "pCloud request timeout").Default("30s").DurationVar(&c.timeout)
}

func (c *storagePCloudFlags) Connect(ctx context.Context, isCreate bool, formatVersion int) (blob.Storage, error) {
	_ = formatVersion

	c.options.Timeout = c.timeout

	if err := c.resolveOAuth(ctx); err != nil {
		return nil, err
	}

	return pcloudblob.NewKopiaStorage(ctx, &c.options, isCreate) //nolint:wrapcheck
}

func (c *storagePCloudFlags) resolveOAuth(ctx context.Context) error {
	if c.options.AccessToken != "" || c.options.AuthToken != "" {
		return nil
	}

	code := c.oauthCode
	callbackHost := pcloudOAuthHostname(c.oauthHostname)

	if c.callbackURL != "" {
		if token, err := pcloud.ParseTokenRedirect(c.callbackURL); err == nil {
			c.options.AccessToken = token.AccessToken
			if token.Hostname != "" {
				c.options.APIHost = pcloudAPIHostFromOAuthHostname(token.Hostname)
			}
			return nil
		}

		parsed, err := pcloud.ParseCodeRedirect(c.callbackURL)
		if err != nil {
			return err
		}
		code = parsed.Code
		if parsed.Hostname != "" {
			callbackHost = pcloudOAuthHostname(parsed.Hostname)
		}
	}

	if code == "" {
		return fmt.Errorf("one of --access-token, --auth-token, --oauth-code or --callback-url is required for pCloud")
	}
	if c.clientID == "" {
		return fmt.Errorf("missing --client-id or PCLOUD_CLIENT_ID for pCloud OAuth code exchange")
	}
	if c.clientSecret == "" {
		return fmt.Errorf("missing --client-secret or PCLOUD_CLIENT_SECRET for pCloud OAuth code exchange")
	}

	apiHost := strings.TrimRight(c.options.APIHost, "/")
	if callbackHost != "" {
		apiHost = pcloudAPIHostFromOAuthHostname(callbackHost)
		c.options.APIHost = apiHost
	}

	exchangeCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	client := pcloud.NewClient(pcloud.Options{APIHost: apiHost, Timeout: c.timeout})
	token, err := client.ExchangeCode(exchangeCtx, pcloud.TokenRequest{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Code:         code,
	})
	if err != nil {
		return wrapPCloudOAuthExchangeError(err)
	}

	c.options.AccessToken = token.AccessToken
	return nil
}

func init() {
	mustRegisterStorageProvider(
		pcloudblob.StorageType,
		"a pCloud folder",
		func() StorageFlags { return &storagePCloudFlags{} },
	)
}
