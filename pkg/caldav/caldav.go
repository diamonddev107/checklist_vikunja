// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-2021 Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public Licensee as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public Licensee for more details.
//
// You should have received a copy of the GNU Affero General Public Licensee
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package caldav

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/models"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/utils"
)

// DateFormat is the caldav date format
const DateFormat = `20060102T150405`

// Event holds a single caldav event
type Event struct {
	Summary     string
	Description string
	UID         string
	Alarms      []Alarm
	Color       string

	Timestamp time.Time
	Start     time.Time
	End       time.Time
}

// Todo holds a single VTODO
type Todo struct {
	// Required
	Timestamp time.Time
	UID       string

	// Optional
	Summary      string
	Description  string
	Completed    time.Time
	Organizer    *user.User
	Priority     int64 // 0-9, 1 is highest
	RelatedToUID string
	Color        string

	Start       time.Time
	End         time.Time
	DueDate     time.Time
	Duration    time.Duration
	RepeatAfter int64
	RepeatMode  models.TaskRepeatMode

	Created time.Time
	Updated time.Time // last-mod
}

// Alarm holds infos about an alarm from a caldav event
type Alarm struct {
	Time        time.Time
	Description string
}

// Config is the caldav calendar config
type Config struct {
	Name   string
	ProdID string
	Color  string
}

func getCaldavColor(color string) (caldavcolor string) {
	if color == "" {
		return ""
	}

	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}

	color += "FF"

	return `
X-APPLE-CALENDAR-COLOR:` + color + `
X-OUTLOOK-COLOR:` + color + `
X-FUNAMBOL-COLOR:` + color
}

// ParseEvents parses an array of caldav events and gives them back as string
func ParseEvents(config *Config, events []*Event) (caldavevents string) {
	caldavevents += `BEGIN:VCALENDAR
VERSION:2.0
METHOD:PUBLISH
X-PUBLISHED-TTL:PT4H
X-WR-CALNAME:` + config.Name + `
PRODID:-//` + config.ProdID + `//EN` + getCaldavColor(config.Color)

	for _, e := range events {

		if e.UID == "" {
			e.UID = makeCalDavTimeFromTimeStamp(e.Timestamp) + utils.Sha256(e.Summary)
		}

		formattedDescription := ""
		if e.Description != "" {
			re := regexp.MustCompile(`\r?\n`)
			formattedDescription = re.ReplaceAllString(e.Description, "\\n")
		}

		caldavevents += `
BEGIN:VEVENT
UID:` + e.UID + `
SUMMARY:` + e.Summary + getCaldavColor(e.Color) + `
DESCRIPTION:` + formattedDescription + `
DTSTAMP:` + makeCalDavTimeFromTimeStamp(e.Timestamp) + `
DTSTART:` + makeCalDavTimeFromTimeStamp(e.Start) + `
DTEND:` + makeCalDavTimeFromTimeStamp(e.End)

		for _, a := range e.Alarms {
			if a.Description == "" {
				a.Description = e.Summary
			}

			caldavevents += `
BEGIN:VALARM
TRIGGER:` + calcAlarmDateFromReminder(e.Start, a.Time) + `
ACTION:DISPLAY
DESCRIPTION:` + a.Description + `
END:VALARM`
		}
		caldavevents += `
END:VEVENT`
	}

	caldavevents += `
END:VCALENDAR` // Need a line break

	return
}

func formatDuration(duration time.Duration) string {
	seconds := duration.Seconds() - duration.Minutes()*60
	minutes := duration.Minutes() - duration.Hours()*60

	return strconv.FormatFloat(duration.Hours(), 'f', 0, 64) + `H` +
		strconv.FormatFloat(minutes, 'f', 0, 64) + `M` +
		strconv.FormatFloat(seconds, 'f', 0, 64) + `S`
}

// ParseTodos returns a caldav vcalendar string with todos
func ParseTodos(config *Config, todos []*Todo) (caldavtodos string) {
	caldavtodos = `BEGIN:VCALENDAR
VERSION:2.0
METHOD:PUBLISH
X-PUBLISHED-TTL:PT4H
X-WR-CALNAME:` + config.Name + `
PRODID:-//` + config.ProdID + `//EN` + getCaldavColor(config.Color)

	for _, t := range todos {
		if t.UID == "" {
			t.UID = makeCalDavTimeFromTimeStamp(t.Timestamp) + utils.Sha256(t.Summary)
		}

		caldavtodos += `
BEGIN:VTODO
UID:` + t.UID + `
DTSTAMP:` + makeCalDavTimeFromTimeStamp(t.Timestamp) + `
SUMMARY:` + t.Summary + getCaldavColor(t.Color)

		if t.Start.Unix() > 0 {
			caldavtodos += `
DTSTART:` + makeCalDavTimeFromTimeStamp(t.Start)
			if t.Duration != 0 && t.DueDate.Unix() == 0 {
				caldavtodos += `
DURATION:PT` + formatDuration(t.Duration)
			}
		}
		if t.End.Unix() > 0 {
			caldavtodos += `
DTEND:` + makeCalDavTimeFromTimeStamp(t.End)
		}
		if t.Description != "" {
			re := regexp.MustCompile(`\r?\n`)
			formattedDescription := re.ReplaceAllString(t.Description, "\\n")
			caldavtodos += `
DESCRIPTION:` + formattedDescription
		}
		if t.Completed.Unix() > 0 {
			caldavtodos += `
COMPLETED:` + makeCalDavTimeFromTimeStamp(t.Completed) + `
STATUS:COMPLETED`
		}
		if t.Organizer != nil {
			caldavtodos += `
ORGANIZER;CN=:` + t.Organizer.Username
		}

		if t.RelatedToUID != "" {
			caldavtodos += `
RELATED-TO:` + t.RelatedToUID
		}

		if t.DueDate.Unix() > 0 {
			caldavtodos += `
DUE:` + makeCalDavTimeFromTimeStamp(t.DueDate)
		}

		if t.Created.Unix() > 0 {
			caldavtodos += `
CREATED:` + makeCalDavTimeFromTimeStamp(t.Created)
		}

		if t.Priority != 0 {
			caldavtodos += `
PRIORITY:` + strconv.Itoa(mapPriorityToCaldav(t.Priority))
		}

		if t.RepeatAfter > 0 || t.RepeatMode == models.TaskRepeatModeMonth {
			if t.RepeatMode == models.TaskRepeatModeMonth {
				caldavtodos += `
RRULE:FREQ=MONTHLY;BYMONTHDAY=` + t.DueDate.Format("02") // Day of the month
			} else {
				caldavtodos += `
RRULE:FREQ=SECONDLY;INTERVAL=` + strconv.FormatInt(t.RepeatAfter, 10)
			}
		}

		caldavtodos += `
LAST-MODIFIED:` + makeCalDavTimeFromTimeStamp(t.Updated)

		caldavtodos += `
END:VTODO`
	}

	caldavtodos += `
END:VCALENDAR` // Need a line break

	return
}

func makeCalDavTimeFromTimeStamp(ts time.Time) (caldavtime string) {
	return ts.In(time.UTC).Format(DateFormat) + "Z"
}

func calcAlarmDateFromReminder(eventStart, reminder time.Time) (alarmTime string) {
	diff := reminder.Sub(eventStart)
	diffStr := strings.ToUpper(diff.String())
	if diff < 0 {
		alarmTime += `-`
		// We append the - at the beginning of the caldav flag, that would get in the way if the minutes
		// themselves are also containing it
		diffStr = diffStr[1:]
	}
	alarmTime += `PT` + diffStr
	return
}
