package service

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/target"
)

func Name(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return "", Invalid("name must contain 1 to 100 characters")
	}
	return name, nil
}

func ValidateConfig(c *models.MonitorConfig) error {
	var err error
	c.Name, err = Name(c.Name)
	if err != nil {
		return err
	}
	if err = target.ValidateURL(c.URL); err != nil {
		return Invalid("url must be an HTTP(S) URL without credentials or a private destination")
	}
	if c.IntervalSeconds < 1 || c.IntervalSeconds > 86400 || c.TimeoutSeconds < 1 || c.TimeoutSeconds > 300 {
		return Invalid("invalid interval or timeout")
	}
	if c.FailureThreshold < 1 || c.FailureThreshold > 100 || c.RecoveryThreshold < 1 || c.RecoveryThreshold > 100 {
		return Invalid("thresholds must be between 1 and 100")
	}
	if c.MaxBodyBytes < 1 || c.MaxBodyBytes > 10485760 || c.MaxRedirects < 0 || c.MaxRedirects > 10 {
		return Invalid("invalid response limits")
	}
	if len(c.ExpectedStatusCodes) == 0 || len(c.ExpectedStatusCodes) > 500 {
		return Invalid("expected_status_codes must contain HTTP status codes")
	}
	c.ExpectedStatusCodes = append([]int64(nil), c.ExpectedStatusCodes...)
	sort.Slice(c.ExpectedStatusCodes, func(i, j int) bool { return c.ExpectedStatusCodes[i] < c.ExpectedStatusCodes[j] })
	codes := make([]int64, 0, len(c.ExpectedStatusCodes))
	for _, code := range c.ExpectedStatusCodes {
		if code < 100 || code > 599 {
			return Invalid("invalid HTTP status code")
		}
		if len(codes) == 0 || codes[len(codes)-1] != code {
			codes = append(codes, code)
		}
	}
	c.ExpectedStatusCodes = codes
	if c.WarningResponseMS != nil && (*c.WarningResponseMS <= 0 || *c.WarningResponseMS > c.TimeoutSeconds*1000) {
		return Invalid("warning threshold must be positive and within timeout")
	}
	switch c.BodyMatchType {
	case "none":
		if c.BodyPattern != nil {
			return Invalid("body_pattern must be null when matching is disabled")
		}
	case "substring", "regexp":
		if c.BodyPattern == nil || *c.BodyPattern == "" || len(*c.BodyPattern) > 4096 {
			return Invalid("body_pattern must contain 1 to 4096 bytes")
		}
		if c.BodyMatchType == "regexp" {
			if _, err = regexp.Compile(*c.BodyPattern); err != nil {
				return Invalid("invalid regular expression")
			}
		}
	default:
		return Invalid("invalid body_match_type")
	}
	return nil
}

func sameConditions(a, b models.MonitorConfig) bool {
	a.Name = ""
	b.Name = ""
	return reflect.DeepEqual(a, b)
}

func ValidPage(page models.Page) bool {
	return page.Limit >= 1 && page.Limit <= 100 && page.Offset >= 0 && page.Offset <= 100000
}
