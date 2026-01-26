package chat

import (
	"fmt"
	"strings"
	"time"
)

// PromptParams contains parameters for generating system prompts
type PromptParams struct {
	Date               time.Time
	Locale             string
	Language           string
	MaxContextLoadTime int      // in minutes
	Platforms          []string // available platforms (e.g., ["telegram", "wechat"])
	CurrentPlatform    string   // current platform the user is using
}

// SystemPrompt generates the system prompt for the AI assistant
// This is migrated from packages/agent/src/prompts/system.ts
func SystemPrompt(params PromptParams) string {
	if params.Language == "" {
		params.Language = "Same as user input"
	}
	if params.MaxContextLoadTime == 0 {
		params.MaxContextLoadTime = 24 * 60 // 24 hours default
	}
	if params.CurrentPlatform == "" {
		params.CurrentPlatform = "client"
	}

	// Build platforms list
	platformsList := ""
	if len(params.Platforms) > 0 {
		lines := make([]string, len(params.Platforms))
		for i, p := range params.Platforms {
			lines[i] = fmt.Sprintf("  - %s", p)
		}
		platformsList = strings.Join(lines, "\n")
	}

	timeStr := FormatTime(params.Date, params.Locale)

	return fmt.Sprintf(`---
%s
language: %s
available-platforms:
%s
current-platform: %s
---
You are a personal housekeeper assistant, which able to manage the master's daily affairs.

Your abilities:
- Long memory: You possess long-term memory; conversations from the last %d minutes will be directly loaded into your context. Additionally, you can use tools to search for past memories.
- Scheduled tasks: You can create scheduled tasks to automatically remind you to do something.
- Messaging: You may allowed to use message software to send messages to the master.

**Response Guidelines**
- Always respond in the language specified above, unless it says "Same as user input", then match the user's language.
- Be helpful, concise, and friendly.
- For complex questions, break down your answer into clear steps.
- If you're unsure about something, acknowledge it honestly.`,
		timeStr,
		params.Language,
		platformsList,
		params.CurrentPlatform,
		params.MaxContextLoadTime,
	)
}

// SchedulePrompt generates a prompt for scheduled task execution
// This is migrated from packages/agent/src/prompts/schedule.ts
type SchedulePromptParams struct {
	Date                time.Time
	Locale              string
	ScheduleName        string
	ScheduleDescription string
	ScheduleID          string
	MaxCalls            *int // nil means unlimited
	CronPattern         string
	Command             string // the natural language command to execute
}

func SchedulePrompt(params SchedulePromptParams) string {
	timeStr := FormatTime(params.Date, params.Locale)

	maxCallsStr := "Unlimited"
	if params.MaxCalls != nil {
		maxCallsStr = fmt.Sprintf("%d", *params.MaxCalls)
	}

	return fmt.Sprintf(`---
notice: **This is a scheduled task automatically send to you by the system, not the user input**
%s
schedule-name: %s
schedule-description: %s
schedule-id: %s
max-calls: %s
cron-pattern: %s
---

**COMMAND**

%s`,
		timeStr,
		params.ScheduleName,
		params.ScheduleDescription,
		params.ScheduleID,
		maxCallsStr,
		params.CronPattern,
		params.Command,
	)
}

// FormatTime formats the date and time according to locale
func FormatTime(date time.Time, locale string) string {
	if locale == "" {
		locale = "en-US"
	}

	// Format date and time
	// For simplicity, using standard format. In production, you might want to use
	// a proper i18n library for locale-specific formatting
	dateStr := date.Format("2006-01-02")
	timeStr := date.Format("15:04:05")

	return fmt.Sprintf("date: %s\ntime: %s", dateStr, timeStr)
}

// Quote wraps content in backticks for markdown code formatting
func Quote(content string) string {
	return fmt.Sprintf("`%s`", content)
}

// Block wraps content in code block with optional language tag
func Block(content, tag string) string {
	if tag == "" {
		return fmt.Sprintf("```\n%s\n```", content)
	}
	return fmt.Sprintf("```%s\n%s\n```", tag, content)
}
