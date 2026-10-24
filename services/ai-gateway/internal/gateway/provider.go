package gateway

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func NewAnalyzerFromEnv() (Analyzer, int, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("PHOTOKEEP_GATEWAY_PROVIDER")))
	if provider == "" {
		provider = "xkiro"
	}
	var analyzer Analyzer
	var defaultConcurrency int
	var err error
	switch provider {
	case "xkiro":
		keys := firstConfigured("XKIRO_API_KEYS", "XKIRO_API_KEY")
		analyzer, err = NewXKiroKeyPool(keys, os.Getenv("XKIRO_MODEL"))
		if _, ok := analyzer.(*XKiroKeyPool); ok {
			defaultConcurrency = 64
		}
	case "cloudflare-thaillm", "cloudflare/thaillm":
		accounts, accountErr := cloudflareAccountsFromEnv()
		if accountErr != nil {
			return nil, 0, accountErr
		}
		keys := splitSecrets(firstConfigured("THAILLM_API_KEYS", "THAILLM_API_KEY"))
		analyzer, err = NewCloudflareThaiLLMAnalyzer(accounts, keys, os.Getenv("THAILLM_MODEL"))
		if pool, ok := analyzer.(*CloudflareThaiLLMAnalyzer); ok {
			defaultConcurrency = pool.DefaultConcurrency()
		}
	default:
		return nil, 0, fmt.Errorf("PHOTOKEEP_GATEWAY_PROVIDER ต้องเป็น xkiro หรือ cloudflare-thaillm")
	}
	if err != nil {
		return nil, 0, err
	}
	fallbackURL, fallbackConfigured := os.LookupEnv("PHOTOKEEP_GATEWAY_FALLBACK_URL")
	fallbackURL = strings.TrimSpace(fallbackURL)
	if !fallbackConfigured && provider == "xkiro" {
		fallbackURL = defaultVisionAnalyzeURL
	}
	if strings.EqualFold(fallbackURL, "off") {
		fallbackURL = ""
	}
	if fallbackURL != "" {
		fallback, fallbackErr := NewRemoteVisionAnalyzer(
			fallbackURL,
			os.Getenv("PHOTOKEEP_GATEWAY_FALLBACK_API_KEY"),
			os.Getenv("PHOTOKEEP_GATEWAY_FALLBACK_API_KEY_HEADER"),
		)
		if fallbackErr != nil {
			return nil, 0, fallbackErr
		}
		analyzer = NewRateLimitFailoverAnalyzer(analyzer, fallback)
	}
	if defaultConcurrency < 1 {
		defaultConcurrency = 1
	}
	if value := strings.TrimSpace(os.Getenv("PHOTOKEEP_GATEWAY_CONCURRENCY")); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 1 || parsed > 128 {
			return nil, 0, errors.New("PHOTOKEEP_GATEWAY_CONCURRENCY ต้องอยู่ระหว่าง 1 ถึง 128")
		}
		defaultConcurrency = parsed
	}
	if defaultConcurrency > 128 {
		defaultConcurrency = 128
	}
	return analyzer, defaultConcurrency, nil
}

func firstConfigured(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func splitSecrets(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' })
	secrets := make([]string, 0, len(parts))
	for _, part := range parts {
		if secret := strings.TrimSpace(part); secret != "" {
			secrets = append(secrets, secret)
		}
	}
	return secrets
}

func cloudflareAccountsFromEnv() ([]CloudflareAccount, error) {
	accounts := make([]CloudflareAccount, 0)
	for _, entry := range splitSecrets(os.Getenv("CLOUDFLARE_ACCOUNTS")) {
		accountID, token, ok := strings.Cut(entry, ":")
		if !ok || strings.TrimSpace(accountID) == "" || strings.TrimSpace(token) == "" {
			return nil, errors.New("CLOUDFLARE_ACCOUNTS ต้องเป็น account_id:token คั่นด้วย comma หรือขึ้นบรรทัดใหม่")
		}
		accounts = append(accounts, CloudflareAccount{AccountID: strings.TrimSpace(accountID), Token: strings.TrimSpace(token)})
	}
	if len(accounts) == 0 {
		accountID, token := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID")), strings.TrimSpace(os.Getenv("CLOUDFLARE_AI_TOKEN"))
		if accountID != "" && token != "" {
			accounts = append(accounts, CloudflareAccount{AccountID: accountID, Token: token})
		}
	}
	if len(accounts) == 0 {
		return nil, errors.New("ตั้งค่า CLOUDFLARE_ACCOUNTS อย่างน้อยหนึ่งบัญชีใน gateway.env")
	}
	return accounts, nil
}
