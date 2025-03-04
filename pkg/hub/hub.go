package hub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/acorn-io/acorn/pkg/config"
	"github.com/acorn-io/baaah/pkg/randomtoken"
	"github.com/pkg/browser"
	"github.com/sirupsen/logrus"
	"k8s.io/utils/strings/slices"
)

func IsHub(ctx context.Context, address string) (bool, error) {
	cfg, err := config.ReadCLIConfig()
	if err != nil {
		return false, err
	}

	if slices.Contains(cfg.HubServers, address) {
		return true, nil
	}

	req, err := http.NewRequest(http.MethodGet, toDiscoverURL(address), nil)
	if err != nil {
		return false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if !strings.Contains(string(data), "TokenRequest") {
		return false, nil
	}

	cfg.HubServers = append(cfg.HubServers, address)
	return true, cfg.Save()
}

func Projects(ctx context.Context, address, token string) (result []string, err error) {
	accounts := &accountList{}
	err = httpGet(ctx, toAccountsURL(address), token, accounts)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts.Items {
		if account.Status.EndpointURL != "" {
			// using account type because we just care about the name
			projects := &accountList{}
			if err := httpGet(ctx, toProjectsURL(account.Status.EndpointURL), token, projects); err == nil {
				for _, project := range projects.Items {
					result = append(result, address+"/"+account.Metadata.Name+"/"+project.Metadata.Name)
				}
			}
		}
	}
	return result, nil
}

func ProjectURLAndNamespace(ctx context.Context, project, token string) (url string, namespace string, err error) {
	address, rest, _ := strings.Cut(project, "/")
	accountName, ns, _ := strings.Cut(rest, "/")

	obj := &account{}
	if err := httpGet(ctx, toAccountURL(address, accountName), token, obj); err != nil {
		return "", "", err
	}
	if obj.Status.EndpointURL == "" {
		return "", "", fmt.Errorf("failed to find endpoint URL for account %s, account may still be creating", accountName)
	}
	return obj.Status.EndpointURL, ns, nil
}

func Login(ctx context.Context, password, address string) (user string, pass string, err error) {
	if password == "" {
		password, err = randomtoken.Generate()
		if err != nil {
			return "", "", err
		}

		url := toLoginURL(address, password)
		_ = browser.OpenURL(url)
		fmt.Printf("\nNavigate your browser to %s and login\n", url)
	}

	tokenRequestURL := toTokenRequestURL(address, password)
	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-timeout:
			return "", "", fmt.Errorf("timeout getting authentication token")
		default:
		}

		tokenRequest := &tokenRequest{}
		if err := httpGet(ctx, tokenRequestURL, "", tokenRequest); err == nil {
			if tokenRequest.Status.Expired {
				return "", "", fmt.Errorf("token request has expired, please try to login again")
			}
			if tokenRequest.Status.Token != "" {
				httpDelete(ctx, tokenRequestURL, tokenRequest.Status.Token)
				return tokenRequest.Spec.AccountName, tokenRequest.Status.Token, nil
			} else {
				logrus.Debugf("tokenRequest.Status.Token is empty")
			}
		} else {
			logrus.Debugf("error getting tokenrequest: %v", err)
		}

		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
	}
}

func DefaultProject(ctx context.Context, address, user, token string) (string, error) {
	desiredDefault := address + "/" + user + "/acorn"
	projects, err := Projects(ctx, address, token)
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", err
	}
	if slices.Contains(projects, desiredDefault) {
		return desiredDefault, nil
	}
	return projects[0], nil
}
