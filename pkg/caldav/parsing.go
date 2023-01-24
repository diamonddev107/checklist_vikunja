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
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"

	ics "github.com/arran4/golang-ical"
)

func GetCaldavTodosForTasks(list *models.ListWithTasksAndBuckets, listTasks []*models.TaskWithComments) string {

	// Make caldav todos from Vikunja todos
	var caldavtodos []*Todo
	for _, t := range listTasks {

		duration := t.EndDate.Sub(t.StartDate)

		caldavtodos = append(caldavtodos, &Todo{
			Timestamp:   t.Updated,
			UID:         t.UID,
			Summary:     t.Title,
			Description: t.Description,
			Completed:   t.DoneAt,
			// Organizer:     &t.CreatedBy, // Disabled until we figure out how this works
			Priority: t.Priority,
			Start:    t.StartDate,
			End:      t.EndDate,
			Created:  t.Created,
			Updated:  t.Updated,
			DueDate:  t.DueDate,
			Duration: duration,
		})
	}

	caldavConfig := &Config{
		Name:   list.Title,
		ProdID: "Vikunja Todo App",
	}

	return ParseTodos(caldavConfig, caldavtodos)
}

func ParseTaskFromVTODO(content string) (vTask *models.Task, err error) {
	parsed, err := ics.ParseCalendar(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	// We put the task details in a map to be able to handle them more easily
	task := make(map[string]string)
	for _, c := range parsed.Components[0].UnknownPropertiesIANAProperties() {
		task[c.IANAToken] = c.Value
	}

	// Parse the priority
	var priority int64
	if _, ok := task["PRIORITY"]; ok {
		priorityParsed, err := strconv.ParseInt(task["PRIORITY"], 10, 64)
		if err != nil {
			return nil, err
		}

		priority = parseVTODOPriority(priorityParsed)
	}

	// Parse the enddate
	duration, _ := time.ParseDuration(task["DURATION"])

	description := strings.ReplaceAll(task["DESCRIPTION"], "\\,", ",")
	description = strings.ReplaceAll(description, "\\n", "\n")

	vTask = &models.Task{
		UID:         task["UID"],
		Title:       task["SUMMARY"],
		Description: description,
		Priority:    priority,
		DueDate:     caldavTimeToTimestamp(task["DUE"]),
		Updated:     caldavTimeToTimestamp(task["DTSTAMP"]),
		StartDate:   caldavTimeToTimestamp(task["DTSTART"]),
		DoneAt:      caldavTimeToTimestamp(task["COMPLETED"]),
	}

	if task["STATUS"] == "COMPLETED" {
		vTask.Done = true
	}

	if duration > 0 && !vTask.StartDate.IsZero() {
		vTask.EndDate = vTask.StartDate.Add(duration)
	}

	return
}

// https://tools.ietf.org/html/rfc5545#section-3.3.5
func caldavTimeToTimestamp(tstring string) time.Time {
	if tstring == "" {
		return time.Time{}
	}

	format := DateFormat

	if strings.HasSuffix(tstring, "Z") {
		format = `20060102T150405Z`
	}

	if len(tstring) == 8 {
		format = `20060102`
	}

	t, err := time.Parse(format, tstring)
	if err != nil {
		log.Warningf("Error while parsing caldav time %s to TimeStamp: %s", tstring, err)
		return time.Time{}
	}
	return t
}
