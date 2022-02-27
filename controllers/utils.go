package controllers

import (
	"context"
	"fmt"
	poddisruptionbudgetupdaterv1alpha1 "pod-disruption-budget-updater/api/v1alpha1"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isNowWithinPeriod(reqLogger logr.Logger, periods *[]poddisruptionbudgetupdaterv1alpha1.PeriodSpec) bool {
	now := time.Now()
	nowTime := time.Date(0, 1, 1, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())

	for _, period := range *periods {
		from, err := time.ParseInLocation("15:04:05", period.From, time.Local)
		if err != nil {
			reqLogger.V(0).Error(err, "unable to parse period timestamp")
			continue
		}
		to, err := time.ParseInLocation("15:04:05", period.To, time.Local)
		if err != nil {
			reqLogger.V(0).Error(err, "unable to parse period timestamp")
			continue
		}
		if !nowTime.Before(from) && nowTime.Before(to) {
			return true
		}
	}
	return false
}

func patchPodDisruptionBudget(reqLogger logr.Logger, ctx context.Context, r *PodDisruptionBudgetUpdaterReconciler, namespace, pdbName string, mergePatch []byte) error {
	if err := r.Client.Patch(ctx, &v1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      pdbName,
		},
	}, client.RawPatch(types.MergePatchType, mergePatch)); err != nil {
		reqLogger.V(0).Error(err, "PodDisruptionBudget update failed")
		return fmt.Errorf("when patcing PodDisruptionBudget: %w", err)
	}
	reqLogger.V(1).Info("PodDisruptionBudget update succeeded")
	return nil
}
