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
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/web"
	"xorm.io/builder"
	"xorm.io/xorm"
)

// LabelTask represents a relation between a label and a task
type LabelTask struct {
	// The unique, numeric id of this label.
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	TaskID int64 `xorm:"bigint INDEX not null" json:"-" param:"listtask"`
	// The label id you want to associate with a task.
	LabelID int64 `xorm:"bigint INDEX not null" json:"label_id" param:"label"`
	// A timestamp when this task was created. You cannot change this value.
	Created time.Time `xorm:"created not null" json:"created"`

	web.CRUDable `xorm:"-" json:"-"`
	web.Rights   `xorm:"-" json:"-"`
}

// TableName makes a pretty table name
func (LabelTask) TableName() string {
	return "label_tasks"
}

// Delete deletes a label on a task
// @Summary Remove a label from a task
// @Description Remove a label from a task. The user needs to have write-access to the list to be able do this.
// @tags labels
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param task path int true "Task ID"
// @Param label path int true "Label ID"
// @Success 200 {object} models.Message "The label was successfully removed."
// @Failure 403 {object} web.HTTPError "Not allowed to remove the label."
// @Failure 404 {object} web.HTTPError "Label not found."
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{task}/labels/{label} [delete]
func (lt *LabelTask) Delete(s *xorm.Session, a web.Auth) (err error) {
	_, err = s.Delete(&LabelTask{LabelID: lt.LabelID, TaskID: lt.TaskID})
	return err
}

// Create adds a label to a task
// @Summary Add a label to a task
// @Description Add a label to a task. The user needs to have write-access to the list to be able do this.
// @tags labels
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param task path int true "Task ID"
// @Param label body models.LabelTask true "The label object"
// @Success 200 {object} models.LabelTask "The created label relation object."
// @Failure 400 {object} web.HTTPError "Invalid label object provided."
// @Failure 403 {object} web.HTTPError "Not allowed to add the label."
// @Failure 404 {object} web.HTTPError "The label does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{task}/labels [put]
func (lt *LabelTask) Create(s *xorm.Session, a web.Auth) (err error) {
	// Check if the label is already added
	exists, err := s.Exist(&LabelTask{LabelID: lt.LabelID, TaskID: lt.TaskID})
	if err != nil {
		return err
	}
	if exists {
		return ErrLabelIsAlreadyOnTask{lt.LabelID, lt.TaskID}
	}

	// Insert it
	_, err = s.Insert(lt)
	if err != nil {
		return err
	}

	err = updateListByTaskID(s, lt.TaskID)
	return
}

// ReadAll gets all labels on a task
// @Summary Get all labels on a task
// @Description Returns all labels which are assicociated with a given task.
// @tags labels
// @Accept json
// @Produce json
// @Param task path int true "Task ID"
// @Param page query int false "The page number. Used for pagination. If not provided, the first page of results is returned."
// @Param per_page query int false "The maximum number of items per page. Note this parameter is limited by the configured maximum of items per page."
// @Param s query string false "Search labels by label text."
// @Security JWTKeyAuth
// @Success 200 {array} models.Label "The labels"
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{task}/labels [get]
func (lt *LabelTask) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	// Check if the user has the right to see the task
	task := Task{ID: lt.TaskID}
	canRead, _, err := task.CanRead(s, a)
	if err != nil {
		return nil, 0, 0, err
	}
	if !canRead {
		return nil, 0, 0, ErrNoRightToSeeTask{lt.TaskID, a.GetID()}
	}

	return getLabelsByTaskIDs(s, &LabelByTaskIDsOptions{
		User:    &user.User{ID: a.GetID()},
		Search:  search,
		Page:    page,
		TaskIDs: []int64{lt.TaskID},
	})
}

// Helper struct, contains the label + its task ID
type labelWithTaskID struct {
	TaskID int64 `json:"-"`
	Label  `xorm:"extends"`
}

// LabelByTaskIDsOptions is a struct to not clutter the function with too many optional parameters.
type LabelByTaskIDsOptions struct {
	User                *user.User
	Search              string
	Page                int
	PerPage             int
	TaskIDs             []int64
	GetUnusedLabels     bool
	GroupByLabelIDsOnly bool
	GetForUser          int64
}

// Helper function to get all labels for a set of tasks
// Used when getting all labels for one task as well when getting all lables
func getLabelsByTaskIDs(s *xorm.Session, opts *LabelByTaskIDsOptions) (ls []*labelWithTaskID, resultCount int, totalEntries int64, err error) {
	// We still need the task ID when we want to get all labels for a task, but because of this, we get the same label
	// multiple times when it is associated to more than one task.
	// Because of this whole thing, we need this extra switch here to only group by Task IDs if needed.
	// Probably not the most ideal solution.
	var groupBy = "labels.id,label_tasks.task_id"
	var selectStmt = "labels.*, label_tasks.task_id"
	if opts.GroupByLabelIDsOnly {
		groupBy = "labels.id"
		selectStmt = "labels.*"
	}

	// Get all labels associated with these tasks
	var labels []*labelWithTaskID
	cond := builder.And(builder.NotNull{"label_tasks.label_id"})
	if len(opts.TaskIDs) > 0 && opts.GetForUser == 0 {
		cond = builder.And(builder.In("label_tasks.task_id", opts.TaskIDs), cond)
	}
	if opts.GetForUser != 0 {
		cond = builder.And(builder.In("label_tasks.task_id",
			builder.
				Select("id").
				From("tasks").
				Where(builder.In("list_id", getUserListsStatement(opts.GetForUser).Select("l.id"))),
		), cond)
	}
	if opts.GetUnusedLabels {
		cond = builder.Or(cond, builder.Eq{"labels.created_by_id": opts.User.ID})
	}

	ids := []int64{}
	if opts.Search != "" {
		vals := strings.Split(opts.Search, ",")
		for _, val := range vals {
			v, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				log.Debugf("Label search string part '%s' is not a number: %s", val, err)
				continue
			}
			ids = append(ids, v)
		}
	}

	if len(ids) > 0 {
		cond = builder.And(cond, builder.In("labels.id", ids))
	} else {
		cond = builder.And(cond, &builder.Like{"labels.title", "%" + opts.Search + "%"})
	}

	limit, start := getLimitFromPageIndex(opts.Page, opts.PerPage)

	query := s.Table("labels").
		Select(selectStmt).
		Join("LEFT", "label_tasks", "label_tasks.label_id = labels.id").
		Where(cond).
		GroupBy(groupBy).
		OrderBy("labels.id ASC")
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	err = query.Find(&labels)
	if err != nil {
		return nil, 0, 0, err
	}

	if len(labels) == 0 {
		return nil, 0, 0, nil
	}

	// Get all created by users
	var userids []int64
	for _, l := range labels {
		userids = append(userids, l.CreatedByID)
	}
	users := make(map[int64]*user.User)
	if len(userids) > 0 {
		err = s.In("id", userids).Find(&users)
		if err != nil {
			return nil, 0, 0, err
		}
	}

	// Obfuscate all user emails
	for _, u := range users {
		u.Email = ""
	}

	// Put it all together
	for in, l := range labels {
		labels[in].CreatedBy = users[l.CreatedByID]
	}

	// Get the total number of entries
	totalEntries, err = s.Table("labels").
		Select("count(DISTINCT labels.id)").
		Join("LEFT", "label_tasks", "label_tasks.label_id = labels.id").
		Where(cond).
		And("labels.title LIKE ?", "%"+opts.Search+"%").
		Count(&Label{})
	if err != nil {
		return nil, 0, 0, err
	}

	return labels, len(labels), totalEntries, err
}

// Create or update a bunch of task labels
func (t *Task) updateTaskLabels(s *xorm.Session, creator web.Auth, labels []*Label) (err error) {

	// If we don't have any new labels, delete everything right away. Saves us some hassle.
	if len(labels) == 0 && len(t.Labels) > 0 {
		_, err = s.Where("task_id = ?", t.ID).
			Delete(LabelTask{})
		return err
	}

	// If we didn't change anything (from 0 to zero) don't do anything.
	if len(labels) == 0 && len(t.Labels) == 0 {
		return nil
	}

	// Make a hashmap of the new labels for easier comparison
	newLabels := make(map[int64]*Label, len(labels))
	for _, newLabel := range labels {
		newLabels[newLabel.ID] = newLabel
	}

	// Get old labels to delete
	var found bool
	var labelsToDelete []int64
	oldLabels := make(map[int64]*Label, len(t.Labels))
	allLabels := t.Labels
	t.Labels = []*Label{} // We re-empty our labels struct here because we want it to be fully empty so we can put in all the actual labels.
	for _, oldLabel := range allLabels {
		found = false
		if newLabels[oldLabel.ID] != nil {
			found = true // If a new label is already in the list with old labels
		}

		// Put all labels which are only on the old list to the trash
		if !found {
			labelsToDelete = append(labelsToDelete, oldLabel.ID)
		} else {
			t.Labels = append(t.Labels, oldLabel)
		}

		// Put it in a list with all old labels, just using the loop here
		oldLabels[oldLabel.ID] = oldLabel
	}

	// Delete all labels not passed
	if len(labelsToDelete) > 0 {
		_, err = s.In("label_id", labelsToDelete).
			And("task_id = ?", t.ID).
			Delete(LabelTask{})
		if err != nil {
			return err
		}
	}

	// Loop through our labels and add them
	for _, l := range labels {
		// Check if the label is already added on the task and only add it if not
		if oldLabels[l.ID] != nil {
			// continue outer loop
			continue
		}

		// Add the new label
		label, err := getLabelByIDSimple(s, l.ID)
		if err != nil {
			return err
		}

		// Check if the user has the rights to see the label he is about to add
		hasAccessToLabel, _, err := label.hasAccessToLabel(s, creator)
		if err != nil {
			return err
		}
		if !hasAccessToLabel {
			user, _ := creator.(*user.User)
			return ErrUserHasNoAccessToLabel{LabelID: l.ID, UserID: user.ID}
		}

		// Insert it
		_, err = s.Insert(&LabelTask{LabelID: l.ID, TaskID: t.ID})
		if err != nil {
			return err
		}
		t.Labels = append(t.Labels, label)
	}

	err = updateListLastUpdated(s, &List{ID: t.ListID})
	return
}

// LabelTaskBulk is a helper struct to update a bunch of labels at once
type LabelTaskBulk struct {
	// All labels you want to update at once.
	Labels []*Label `json:"labels"`
	TaskID int64    `json:"-" param:"listtask"`

	web.CRUDable `json:"-"`
	web.Rights   `json:"-"`
}

// Create updates a bunch of labels on a task at once
// @Summary Update all labels on a task.
// @Description Updates all labels on a task. Every label which is not passed but exists on the task will be deleted. Every label which does not exist on the task will be added. All labels which are passed and already exist on the task won't be touched.
// @tags labels
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param label body models.LabelTaskBulk true "The array of labels"
// @Param taskID path int true "Task ID"
// @Success 200 {object} models.LabelTaskBulk "The updated labels object."
// @Failure 400 {object} web.HTTPError "Invalid label object provided."
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{taskID}/labels/bulk [post]
func (ltb *LabelTaskBulk) Create(s *xorm.Session, a web.Auth) (err error) {
	task, err := GetTaskByIDSimple(s, ltb.TaskID)
	if err != nil {
		return
	}
	labels, _, _, err := getLabelsByTaskIDs(s, &LabelByTaskIDsOptions{
		TaskIDs: []int64{ltb.TaskID},
	})
	if err != nil {
		return err
	}
	for _, l := range labels {
		task.Labels = append(task.Labels, &l.Label)
	}
	return task.updateTaskLabels(s, a, ltb.Labels)
}
