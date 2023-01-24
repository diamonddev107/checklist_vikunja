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

package models

import (
	"time"

	"code.vikunja.io/api/pkg/utils"
	"xorm.io/builder"

	"code.vikunja.io/api/pkg/notifications"

	"code.vikunja.io/api/pkg/db"
	"xorm.io/xorm"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/cron"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/user"
)

// TaskReminder holds a reminder on a task
type TaskReminder struct {
	ID       int64     `xorm:"bigint autoincr not null unique pk"`
	TaskID   int64     `xorm:"bigint not null INDEX"`
	Reminder time.Time `xorm:"DATETIME not null INDEX 'reminder'"`
	Created  time.Time `xorm:"created not null"`
}

// TableName returns a pretty table name
func (TaskReminder) TableName() string {
	return "task_reminders"
}

type taskUser struct {
	Task *Task      `xorm:"extends"`
	User *user.User `xorm:"extends"`
}

const dbTimeFormat = `2006-01-02 15:04:05`

func getTaskUsersForTasks(s *xorm.Session, taskIDs []int64, cond builder.Cond) (taskUsers []*taskUser, err error) {
	if len(taskIDs) == 0 {
		return
	}

	// Get all creators of tasks
	creators := make(map[int64]*user.User, len(taskIDs))
	err = s.
		Select("users.id, users.username, users.email, users.name").
		Join("LEFT", "tasks", "tasks.created_by_id = users.id").
		In("tasks.id", taskIDs).
		Where(cond).
		GroupBy("tasks.id, users.id, users.username, users.email, users.name").
		Find(&creators)
	if err != nil {
		return
	}

	taskMap := make(map[int64]*Task, len(taskIDs))
	err = s.In("id", taskIDs).Find(&taskMap)
	if err != nil {
		return
	}

	for _, taskID := range taskIDs {
		u, exists := creators[taskMap[taskID].CreatedByID]
		if !exists {
			continue
		}

		taskUsers = append(taskUsers, &taskUser{
			Task: taskMap[taskID],
			User: u,
		})
	}

	var assignees []*TaskAssigneeWithUser
	err = s.Table("task_assignees").
		Select("task_id, users.*").
		In("task_id", taskIDs).
		Join("INNER", "users", "task_assignees.user_id = users.id").
		Where(cond).
		Find(&assignees)
	if err != nil {
		return
	}

	for _, assignee := range assignees {
		taskUsers = append(taskUsers, &taskUser{
			Task: taskMap[assignee.TaskID],
			User: &assignee.User,
		})
	}

	return
}

func getTasksWithRemindersInTheNextMinute(s *xorm.Session, now time.Time) (taskIDs []int64, err error) {
	now = utils.GetTimeWithoutNanoSeconds(now)

	nextMinute := now.Add(1 * time.Minute)

	log.Debugf("[Task Reminder Cron] Looking for reminders between %s and %s to send...", now, nextMinute)

	reminders := []*TaskReminder{}
	err = s.
		Join("INNER", "tasks", "tasks.id = task_reminders.task_id").
		Where("reminder >= ? and reminder < ?", now.Format(dbTimeFormat), nextMinute.Format(dbTimeFormat)).
		And("tasks.done = false").
		Find(&reminders)
	if err != nil {
		return
	}

	log.Debugf("[Task Reminder Cron] Found %d reminders", len(reminders))

	if len(reminders) == 0 {
		return
	}

	// We're sending a reminder to everyone who is assigned to the task or has created it.
	for _, r := range reminders {
		taskIDs = append(taskIDs, r.TaskID)
	}

	return
}

// RegisterReminderCron registers a cron function which runs every minute to check if any reminders are due the
// next minute to send emails.
func RegisterReminderCron() {
	if !config.ServiceEnableEmailReminders.GetBool() {
		return
	}

	if !config.MailerEnabled.GetBool() {
		log.Info("Mailer is disabled, not sending reminders per mail")
		return
	}

	tz := config.GetTimeZone()

	log.Debugf("[Task Reminder Cron] Timezone is %s", tz)

	err := cron.Schedule("* * * * *", func() {
		s := db.NewSession()
		defer s.Close()

		now := time.Now()
		taskIDs, err := getTasksWithRemindersInTheNextMinute(s, now)
		if err != nil {
			log.Errorf("[Task Reminder Cron] Could not get tasks with reminders in the next minute: %s", err)
			return
		}

		if len(taskIDs) == 0 {
			return
		}

		users, err := getTaskUsersForTasks(s, taskIDs, builder.Eq{"users.email_reminders_enabled": true})
		if err != nil {
			log.Errorf("[Task Reminder Cron] Could not get task users to send them reminders: %s", err)
			return
		}

		log.Debugf("[Task Reminder Cron] Sending reminders to %d users", len(users))

		for _, u := range users {
			n := &ReminderDueNotification{
				User: u.User,
				Task: u.Task,
			}

			err = notifications.Notify(u.User, n)
			if err != nil {
				log.Errorf("[Task Reminder Cron] Could not notify user %d: %s", u.User.ID, err)
				return
			}

			log.Debugf("[Task Reminder Cron] Sent reminder email for task %d to user %d", u.Task.ID, u.User.ID)
		}
	})
	if err != nil {
		log.Fatalf("Could not register reminder cron: %s", err)
	}
}
