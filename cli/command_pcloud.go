package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	pcloud "github.com/kopia-backends/go-pcloud"
	"golang.org/x/term"
)

type commandPCloud struct {
	authorize commandPCloudAuthorize
	storage   commandPCloudStorage
}

func (c *commandPCloud) setup(svc appServices, parent commandParent) {
	cmd := parent.Command("pcloud", "pCloud helper commands.")

	c.authorize.setup(svc, cmd)
	c.storage.setup(svc, cmd)
}

type commandPCloudAuthorize struct {
	svc            appServices
	clientID       string
	clientSecret   string
	code           string
	callbackURL    string
	oauthHostname  string
	redirectURI    string
	state          string
	apiHost        string
	envFile        string
	tokenFlow      bool
	forceReapprove bool
	interactive    bool
	timeout        time.Duration
}

func (c *commandPCloudAuthorize) setup(svc appServices, parent commandParent) {
	c.svc = svc

	cmd := parent.Command("authorize", "Generate or exchange pCloud OAuth credentials.")
	cmd.Flag("client-id", "pCloud OAuth client id").Envar(svc.EnvName("PCLOUD_CLIENT_ID")).StringVar(&c.clientID)
	cmd.Flag("client-secret", "pCloud OAuth client secret").Envar(svc.EnvName("PCLOUD_CLIENT_SECRET")).StringVar(&c.clientSecret)
	cmd.Flag("code", "Authorization code returned by pCloud").StringVar(&c.code)
	cmd.Flag("callback-url", "Full callback URL returned by pCloud").StringVar(&c.callbackURL)
	cmd.Flag("oauth-hostname", "pCloud OAuth hostname displayed with an authorization code").Envar(svc.EnvName("PCLOUD_OAUTH_HOSTNAME")).StringVar(&c.oauthHostname)
	cmd.Flag("hostname", "Alias for --oauth-hostname").Hidden().StringVar(&c.oauthHostname)
	cmd.Flag("redirect-uri", "OAuth redirect URI").Envar(svc.EnvName("PCLOUD_REDIRECT_URI")).StringVar(&c.redirectURI)
	cmd.Flag("state", "OAuth state").Envar(svc.EnvName("PCLOUD_OAUTH_STATE")).StringVar(&c.state)
	cmd.Flag("api-host", "pCloud API endpoint used for code exchange").Default(pcloud.DefaultAPIHost).Envar(svc.EnvName("PCLOUD_API_HOST")).StringVar(&c.apiHost)
	cmd.Flag("env-file", "Write resulting pCloud OAuth environment variables to this file instead of stdout").StringVar(&c.envFile)
	cmd.Flag("token-flow", "Use OAuth implicit token flow instead of code flow").BoolVar(&c.tokenFlow)
	cmd.Flag("force-reapprove", "Force pCloud approval screen").BoolVar(&c.forceReapprove)
	cmd.Flag("interactive", "Prompt for missing pCloud OAuth values and callback URL").BoolVar(&c.interactive)
	cmd.Flag("timeout", "pCloud OAuth request timeout").Default("30s").DurationVar(&c.timeout)
	cmd.Action(svc.noRepositoryAction(c.run))
}

func (c *commandPCloudAuthorize) run(ctx context.Context) error {
	if c.interactive {
		if err := c.promptInteractive(); err != nil {
			return err
		}
	}

	if c.clientID == "" {
		return fmt.Errorf("missing --client-id or PCLOUD_CLIENT_ID")
	}

	if c.callbackURL != "" {
		if token, err := pcloud.ParseTokenRedirect(c.callbackURL); err == nil {
			return c.emitToken(token)
		}
	}

	code := c.code
	callbackHost := pcloudOAuthHostname(c.oauthHostname)

	if c.callbackURL != "" && code == "" {
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
		responseType := pcloud.OAuthResponseCode
		if c.tokenFlow {
			responseType = pcloud.OAuthResponseToken
		}

		authURL, err := pcloud.AuthorizeURL(pcloud.AuthorizeOptions{
			ClientID:       c.clientID,
			RedirectURI:    c.redirectURI,
			State:          c.state,
			ResponseType:   responseType,
			ForceReapprove: c.forceReapprove,
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(c.svc.stdout(), authURL) //nolint:errcheck
		return nil
	}

	if c.clientSecret == "" {
		return fmt.Errorf("missing --client-secret or PCLOUD_CLIENT_SECRET")
	}

	apiHost := strings.TrimRight(c.apiHost, "/")
	if callbackHost != "" {
		apiHost = pcloudAPIHostFromOAuthHostname(callbackHost)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	client := pcloud.NewClient(pcloud.Options{APIHost: apiHost, Timeout: c.timeout})
	token, err := client.ExchangeCode(ctx, pcloud.TokenRequest{
		ClientID:     c.clientID,
		ClientSecret: c.clientSecret,
		Code:         code,
	})
	if err != nil {
		return wrapPCloudOAuthExchangeError(err)
	}

	if callbackHost != "" {
		token.Hostname = callbackHost
	}

	return c.emitToken(token)
}

func (c *commandPCloudAuthorize) emitToken(token *pcloud.OAuthToken) error {
	if c.envFile == "" {
		printPCloudOAuthToken(c.svc.stdout(), token)
		return nil
	}

	if err := writePCloudOAuthEnvFile(c.envFile, token); err != nil {
		return err
	}

	fmt.Fprintf(c.svc.stdout(), "pCloud OAuth environment written: %s\n", c.envFile) //nolint:errcheck
	return nil
}

func (c *commandPCloudAuthorize) promptInteractive() error {
	reader := bufio.NewReader(c.svc.stdin())

	if c.clientID == "" {
		clientID, err := promptPCloudRequiredLine(c.svc.stdout(), reader, "pCloud client id: ", "pCloud client id")
		if err != nil {
			return err
		}
		c.clientID = clientID
	}

	if c.clientSecret == "" && !c.tokenFlow {
		clientSecret, err := promptPCloudRequiredSecret(c.svc.stdout(), reader, "pCloud client secret: ", "pCloud client secret")
		if err != nil {
			return err
		}
		c.clientSecret = clientSecret
	}

	if c.callbackURL != "" {
		return nil
	}
	if c.code != "" {
		return c.promptOAuthHostname(reader)
	}

	if c.tokenFlow && c.redirectURI == "" {
		redirectURI, err := promptPCloudRequiredLine(c.svc.stdout(), reader, "pCloud redirect URI: ", "pCloud redirect URI")
		if err != nil {
			return err
		}
		c.redirectURI = redirectURI
	}

	responseType := pcloud.OAuthResponseCode
	if c.tokenFlow {
		responseType = pcloud.OAuthResponseToken
	}

	authURL, err := pcloud.AuthorizeURL(pcloud.AuthorizeOptions{
		ClientID:       c.clientID,
		RedirectURI:    c.redirectURI,
		State:          c.state,
		ResponseType:   responseType,
		ForceReapprove: c.forceReapprove,
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(c.svc.stdout(), "Open this URL in your browser:") //nolint:errcheck
	fmt.Fprintln(c.svc.stdout(), authURL)                          //nolint:errcheck

	callbackOrCode, err := promptPCloudLine(c.svc.stdout(), reader, "Paste pCloud callback URL (recommended) or authorization code: ")
	if err != nil {
		return err
	}
	if callbackOrCode == "" {
		return fmt.Errorf("missing pCloud callback URL or authorization code")
	}

	if strings.Contains(callbackOrCode, "://") {
		c.callbackURL = callbackOrCode
		return nil
	}

	c.code = callbackOrCode

	return c.promptOAuthHostname(reader)
}

func (c *commandPCloudAuthorize) promptOAuthHostname(reader *bufio.Reader) error {
	if c.tokenFlow || c.oauthHostname != "" {
		return nil
	}

	hostname, err := promptPCloudLine(c.svc.stdout(), reader, "pCloud hostname, if shown (api.pcloud.com/eapi.pcloud.com; empty keeps --api-host): ")
	if err != nil {
		return err
	}
	c.oauthHostname = hostname
	return nil
}

func pcloudAPIHostFromOAuthHostname(hostname string) string {
	hostname = pcloudOAuthHostname(hostname)
	if hostname == "" {
		return ""
	}
	return "https://" + hostname
}

func pcloudOAuthHostname(raw string) string {
	hostname := strings.TrimRight(strings.TrimSpace(raw), "/")
	if hostname == "" {
		return ""
	}

	if strings.Contains(hostname, "://") {
		u, err := url.Parse(hostname)
		if err == nil && u.Host != "" {
			hostname = u.Host
		}
	}

	hostname = strings.TrimPrefix(hostname, "//")
	if i := strings.IndexAny(hostname, "/?#"); i >= 0 {
		hostname = hostname[:i]
	}
	return strings.TrimSpace(hostname)
}

func promptPCloudLine(w io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(w, prompt) //nolint:errcheck

	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimSpace(value), nil
}

func promptPCloudRequiredLine(w io.Writer, reader *bufio.Reader, prompt, name string) (string, error) {
	value, err := promptPCloudLine(w, reader, prompt)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("missing %s", name)
	}
	return value, nil
}

func wrapPCloudOAuthExchangeError(err error) error {
	var apiErr *pcloud.APIError
	if errors.As(err, &apiErr) && apiErr.Code == 2012 {
		return fmt.Errorf("pCloud OAuth code exchange failed: %w. Use a fresh code because pCloud authorization codes are one-time use; paste the full callback URL when possible; verify PCLOUD_CLIENT_ID and PCLOUD_CLIENT_SECRET match the same pCloud app", err)
	}

	return err
}

func promptPCloudSecret(w io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	if fd, err := intFd(os.Stdin); err == nil && term.IsTerminal(fd) {
		secret, err := askPass(w, prompt)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(secret), nil
	}

	return promptPCloudLine(w, reader, prompt)
}

func promptPCloudRequiredSecret(w io.Writer, reader *bufio.Reader, prompt, name string) (string, error) {
	value, err := promptPCloudSecret(w, reader, prompt)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("missing %s", name)
	}
	return value, nil
}

func printPCloudOAuthToken(w io.Writer, token *pcloud.OAuthToken) {
	for _, line := range pcloudOAuthEnvLines(token) {
		fmt.Fprintln(w, line) //nolint:errcheck
	}
}

func writePCloudOAuthEnvFile(file string, token *pcloud.OAuthToken) error {
	if strings.TrimSpace(file) == "" {
		return fmt.Errorf("missing --env-file")
	}

	var b strings.Builder
	b.WriteString("# Generated by kopia-pcloud pcloud authorize.\n")
	for _, line := range pcloudOAuthEnvLines(token) {
		b.WriteString(line)
		b.WriteByte('\n')
	}

	return os.WriteFile(file, []byte(b.String()), 0o600)
}

func pcloudOAuthEnvLines(token *pcloud.OAuthToken) []string {
	lines := []string{
		"PCLOUD_ACCESS_TOKEN=" + shellQuotePCloudEnv(token.AccessToken),
	}
	if token.TokenType != "" {
		lines = append(lines, "PCLOUD_TOKEN_TYPE="+shellQuotePCloudEnv(token.TokenType))
	}
	if token.UID != 0 {
		lines = append(lines, fmt.Sprintf("PCLOUD_UID=%d", token.UID))
	}
	if token.Hostname != "" {
		lines = append(lines, "PCLOUD_API_HOST="+shellQuotePCloudEnv("https://"+token.Hostname))
	}
	return lines
}

func shellQuotePCloudEnv(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
