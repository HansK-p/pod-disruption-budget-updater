package controllers

import (
	"fmt"
	poddisruptionbudgetupdaterv1alpha1 "pod-disruption-budget-updater/api/v1alpha1"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type UpdateTask struct {
	job *gocron.Job
}

type UpdateSchedule struct {
	UpdateTasksMap map[string]*UpdateTask
}

type UpdateScheduleKey struct {
	Namespace string
	PDBUName  string
}

type UpdateScheduler struct {
	SchedulesMap map[UpdateScheduleKey]*UpdateSchedule
	scheduler    *gocron.Scheduler
	EventChannel chan event.GenericEvent
}

func (us *UpdateScheduler) updateSchedule(reqLogger logr.Logger, pdbu *poddisruptionbudgetupdaterv1alpha1.PodDisruptionBudgetUpdater) {
	namespace, pdbuName := pdbu.Namespace, pdbu.Name
	reqLogger = reqLogger.WithValues("Function", "updateSchedule", "Namespace", namespace, "PodDistributionBudget", pdbuName)
	reqLogger.V(1).Info("Updating the scheduler")
	if us.SchedulesMap == nil {
		us.SchedulesMap = map[UpdateScheduleKey]*UpdateSchedule{}
	}
	key := UpdateScheduleKey{
		Namespace: namespace,
		PDBUName:  pdbuName,
	}
	var utm *map[string]*UpdateTask
	if updateSchedule, found := us.SchedulesMap[key]; !found {
		updateSchedule = &UpdateSchedule{UpdateTasksMap: map[string]*UpdateTask{}}
		us.SchedulesMap[key] = updateSchedule
		utm = &updateSchedule.UpdateTasksMap
	} else {
		utm = &updateSchedule.UpdateTasksMap
	}
	utmCopy := map[string]*UpdateTask{}
	for key, value := range *utm {
		utmCopy[key] = value
	}
	for idx := range pdbu.Spec.Rules {
		rule := &pdbu.Spec.Rules[idx]
		reqLogger.V(3).Info("Looking at a rule")
		for idx := range rule.Periods {
			period := &rule.Periods[idx]
			reqLogger := reqLogger.WithValues("Period", period)
			reqLogger.V(3).Info("Looking at a period")
			for _, timeStr := range []string{period.From, period.To} {
				reqLogger := reqLogger.WithValues("Time of day", timeStr)
				if _, found := (*utm)[timeStr]; found {
					delete(utmCopy, timeStr)
				} else {
					gocronJob, err := us.scheduler.Every(1).Day().At(timeStr).Do(func() {
						reqLogger.V(1).Info("Running scheduled task")
						us.EventChannel <- event.GenericEvent{
							Object: &poddisruptionbudgetupdaterv1alpha1.PodDisruptionBudgetUpdater{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: namespace,
									Name:      pdbuName,
								},
							},
						}
					})
					if err != nil {
						reqLogger.V(0).Error(err, "unable to schedule task")
					} else {
						reqLogger = reqLogger.WithValues("First Job Run", gocronJob.NextRun().Format(time.RFC3339))
					}
					(*utm)[timeStr] = &UpdateTask{job: gocronJob}
					reqLogger.V(1).Info("Added period task")
				}
			}
		}
	}
	reqLogger.V(3).Info("Remove all triggers no longer in use")
	for timeStr, updateTask := range utmCopy {
		reqLogger := reqLogger.WithValues("Time", timeStr)
		reqLogger.V(1).Info("Remove trigger as it is no longer needed")
		us.scheduler.RemoveByReference(updateTask.job)
		delete(*utm, timeStr)
	}
}

func (us *UpdateScheduler) removeSchedule(reqLogger logr.Logger, namespace, pdbuName string) {
	key := UpdateScheduleKey{
		Namespace: namespace,
		PDBUName:  pdbuName,
	}
	updateSchedule, found := us.SchedulesMap[key]
	if !found {
		reqLogger.V(0).Error(fmt.Errorf("no schedules found for PodDistruptionBudgetUpdater %s/%s", namespace, pdbuName), "No schedules found to delete")
		return
	}
	reqLogger.V(1).Info("Remove all tasks")
	for timeStr, updateTask := range updateSchedule.UpdateTasksMap {
		reqLogger := reqLogger.WithValues("Time", timeStr)
		reqLogger.V(1).Info("Remove scheduler")
		us.scheduler.RemoveByReference(updateTask.job)
	}
	delete(us.SchedulesMap, key)
}

var (
	updateScheduler = UpdateScheduler{}
)

func init() {
	updateScheduler.EventChannel = make(chan event.GenericEvent)
	updateScheduler.scheduler = gocron.NewScheduler(time.Local)
	updateScheduler.scheduler.StartAsync()
}
