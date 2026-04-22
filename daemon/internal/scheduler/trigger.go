package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func NextDueAfter(trigger Trigger, after time.Time) (*time.Time, error) {
	switch trigger.Kind {
	case TriggerKindOnce:
		if trigger.FireAt == nil {
			return nil, fmt.Errorf("one-time trigger requires fireAt")
		}
		fireAt := trigger.FireAt.UTC()
		if fireAt.After(after.UTC()) {
			return &fireAt, nil
		}
		return nil, nil
	case TriggerKindCron:
		return nextCronDueAfter(trigger.CronExpr, trigger.Timezone, after)
	default:
		return nil, fmt.Errorf("unsupported trigger kind %q", trigger.Kind)
	}
}

func nextCronDueAfter(expr, timezone string, after time.Time) (*time.Time, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, fmt.Errorf("cron expression is required")
	}
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields")
	}
	minutes, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("parse minute field: %w", err)
	}
	hours, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("parse hour field: %w", err)
	}
	days, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("parse day-of-month field: %w", err)
	}
	months, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("parse month field: %w", err)
	}
	weekdays, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("parse weekday field: %w", err)
	}

	cursor := after.In(location).Add(time.Minute).Truncate(time.Minute)
	deadline := cursor.AddDate(1, 0, 0)
	for !cursor.After(deadline) {
		if minutes[cursor.Minute()] &&
			hours[cursor.Hour()] &&
			days[cursor.Day()] &&
			months[int(cursor.Month())] &&
			weekdays[int(cursor.Weekday())] {
			due := cursor.UTC()
			return &due, nil
		}
		cursor = cursor.Add(time.Minute)
	}
	return nil, fmt.Errorf("no matching cron time found within one year")
}

func parseCronField(field string, min, max int) (map[int]bool, error) {
	allowed := make(map[int]bool)
	for _, part := range strings.Split(strings.TrimSpace(field), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty cron field component")
		}
		if part == "*" {
			for value := min; value <= max; value++ {
				allowed[value] = true
			}
			continue
		}
		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step %q", part)
			}
			for value := min; value <= max; value += step {
				allowed[value] = true
			}
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			start, err := strconv.Atoi(bounds[0])
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q", part)
			}
			end, err := strconv.Atoi(bounds[1])
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q", part)
			}
			if start > end || start < min || end > max {
				return nil, fmt.Errorf("range %q out of bounds", part)
			}
			for value := start; value <= end; value++ {
				allowed[value] = true
			}
			continue
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", part)
		}
		if value < min || value > max {
			return nil, fmt.Errorf("value %d out of bounds", value)
		}
		allowed[value] = true
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("empty cron field %q", field)
	}
	return allowed, nil
}
