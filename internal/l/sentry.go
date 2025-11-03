package l

import "github.com/getsentry/sentry-go"

func Sentry() *sentry.Hub {
	return sentry.CurrentHub()
}

func SentryWithUser(userId string) *sentry.Hub {
	hub := Sentry().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{ID: userId})
	})
	return hub
}

func SentryWithTag(tag, val string) *sentry.Hub {
	hub := Sentry().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetTag(tag, val)
	})
	return hub
}

func SentryWithUserAndTag(userId, tag, val string) *sentry.Hub {
	hub := Sentry().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{ID: userId})
		scope.SetTag(tag, val)
	})
	return hub
}
