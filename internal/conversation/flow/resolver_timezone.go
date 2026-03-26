package flow

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/memohai/memoh/internal/db"
	"github.com/memohai/memoh/internal/timezone"
)

// resolveTimezone resolves the effective timezone for a request.
// Priority: bot timezone > user timezone > system default.
func (r *Resolver) resolveTimezone(ctx context.Context, botID, userID string) (string, *time.Location) {
	fallbackName, fallbackLocation := r.systemTimezoneDefaults()

	// 1. Try bot timezone first.
	if name, loc, ok := r.loadBotTimezone(ctx, botID); ok {
		return name, loc
	}

	// 2. Fall back to user timezone.
	if name, loc, ok := r.loadUserTimezone(ctx, userID); ok {
		return name, loc
	}

	return fallbackName, fallbackLocation
}

func (r *Resolver) systemTimezoneDefaults() (string, *time.Location) {
	if r.clockLocation != nil {
		return r.clockLocation.String(), r.clockLocation
	}
	return timezone.DefaultName, timezone.MustResolve(timezone.DefaultName)
}

func (r *Resolver) loadBotTimezone(ctx context.Context, botID string) (string, *time.Location, bool) {
	if r.queries == nil || strings.TrimSpace(botID) == "" {
		return "", nil, false
	}
	botUUID, err := db.ParseUUID(botID)
	if err != nil {
		return "", nil, false
	}
	row, err := r.queries.GetBotByID(ctx, botUUID)
	if err != nil {
		return "", nil, false
	}
	tz := ""
	if row.Timezone.Valid {
		tz = strings.TrimSpace(row.Timezone.String)
	}
	if tz == "" {
		return "", nil, false
	}
	loc, name, err := timezone.Resolve(tz)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("resolve bot timezone failed",
				slog.String("bot_id", botID),
				slog.String("timezone", tz),
				slog.Any("error", err),
			)
		}
		return "", nil, false
	}
	return name, loc, true
}

func (r *Resolver) loadUserTimezone(ctx context.Context, userID string) (string, *time.Location, bool) {
	if r.accountService == nil || strings.TrimSpace(userID) == "" {
		return "", nil, false
	}
	account, err := r.accountService.Get(ctx, strings.TrimSpace(userID))
	if err != nil {
		return "", nil, false
	}
	tz := strings.TrimSpace(account.Timezone)
	if tz == "" {
		return "", nil, false
	}
	loc, name, err := timezone.Resolve(tz)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("resolve user timezone failed",
				slog.String("user_id", userID),
				slog.String("timezone", tz),
				slog.Any("error", err),
			)
		}
		return "", nil, false
	}
	return name, loc, true
}
