package notify

// This file is a fork extension: it removes the secrets of the DNS providers and of the notifications from the errors in events
// Upstream dropped this redaction in the review of the notifications, but the detailed messages (detail.go) show the error of each unhealthy endpoint, so the fork keeps it
// Upstream code calls it from one place only: newNotifier sets Notifier.redact
// To drop it, delete this file and its test, then remove the field and that call, see FORK.md

import (
	"reflect"
	"regexp"

	"github.com/italypaleale/ddup/pkg/config"
)

var secretFieldRegexp = regexp.MustCompile(`(?i)token|secret|key|password`)

// newForkRedactor returns the redactor for the errors of events: the secrets of the providers, the webhooks and the emails
func newForkRedactor(cfg *config.Config) *redactor {
	secrets := make([]string, 0, 8)
	for _, p := range cfg.Providers {
		secrets = append(secrets, providerSecrets(p)...)
	}
	for _, c := range cfg.Notifications.Webhooks {
		secrets = append(secrets, urlSecrets(c.URL)...)
		secrets = append(secrets, c.Token)
	}
	for _, c := range cfg.Notifications.Email {
		secrets = append(secrets, connStringSecrets(c.ConnectionString)...)
	}
	return newRedactor(secrets...)
}

// providerSecrets returns the values of the settings of a DNS provider that look like secrets (tokens, keys, and similar)
func providerSecrets(provider any) []string {
	val := reflect.ValueOf(provider)
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	var secrets []string
	for i := range val.NumField() {
		field := val.Field(i)
		switch {
		case field.Kind() == reflect.Pointer && !field.IsNil() && field.Elem().Kind() == reflect.Struct:
			secrets = append(secrets, providerSecrets(field.Interface())...)
		case field.Kind() == reflect.String && secretFieldRegexp.MatchString(val.Type().Field(i).Name):
			secrets = append(secrets, field.String())
		}
	}
	return secrets
}
