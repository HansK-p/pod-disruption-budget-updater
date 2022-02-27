package controllers

import (
	"context"
	"fmt"
	poddisruptionbudgetupdaterv1alpha1 "pod-disruption-budget-updater/api/v1alpha1"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func reconcileCrd(reqLogger logr.Logger, ctx context.Context, r *PodDisruptionBudgetUpdaterReconciler, request reconcile.Request, crd *poddisruptionbudgetupdaterv1alpha1.PodDisruptionBudgetUpdater) error {
	namespace := crd.Namespace
	crdName := crd.Name
	pdbNames := crd.Spec.PodDisruptionBudgets
	reqLogger = reqLogger.WithValues("CRD.Namespace", namespace, "CRD.Name", crdName)

	reqLogger.V(4).Info("Start reconcileCrd", "status", crd.Status)

	reqLogger.V(3).Info("The request has to match this CRD or something we watch for us to actually do a job")
	isRelevant := false

	reqName := request.Name
	if reqName == crdName {
		isRelevant = true
		reqLogger.Info("The requests affects the CRD and is relevant for us")
	} else {
		for _, pdbName := range pdbNames {
			if reqName == pdbName {
				isRelevant = true
				reqLogger.Info("The request affects a PodDistributionBudget we manage and is relevant for us")
			}
		}
	}
	if !isRelevant {
		reqLogger.V(3).Info("The request is not relevant as the request object isn't related to a PodDistributionBudget we manage or the CRD itself")
		return nil
	}

	updateScheduler.updateSchedule(reqLogger, crd)

	for _, pdbName := range crd.Spec.PodDisruptionBudgets {
		reqLogger := reqLogger.WithValues("Name", pdbName)
		pdb := policyv1.PodDisruptionBudget{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: pdbName}, &pdb); err != nil {
			reqLogger.V(0).Error(err, "When trying to find PodDisruptionBudget")
			continue
		}
		targetMinAvailable := crd.Spec.Default.Settings.MinAvailable
		for idx := range crd.Spec.Rules {
			rule := &crd.Spec.Rules[idx]
			if isNowWithinPeriod(reqLogger, &rule.Periods) {
				targetMinAvailable = rule.Settings.MinAvailable
			}
		}
		if targetMinAvailable == nil {
			reqLogger.V(1).Info("no value found for TargetMinAvailable, so nothing to update")
			continue
		}
		if minAvailable := pdb.Spec.MinAvailable; *minAvailable != *targetMinAvailable {
			reqLogger := reqLogger.WithValues("NewMinAvailable", *targetMinAvailable)
			var mergePatch []byte
			if targetMinAvailable.Type == intstr.Int {
				mergePatch = []byte(fmt.Sprintf(`{"spec":{"minAvailable":%d}}`, targetMinAvailable.IntVal))
			} else {
				mergePatch = []byte(fmt.Sprintf(`{"spec":{"minAvailable":"%s"}}`, targetMinAvailable.StrVal))
			}
			reqLogger = reqLogger.WithValues("MergePatch", string(mergePatch))
			reqLogger.V(1).Info("Patching")
			if err := patchPodDisruptionBudget(reqLogger, ctx, r, namespace, pdbName, mergePatch); err != nil {
				return fmt.Errorf("when patching PodDistributionBudget '%s' in namespace '%s' using patch '%s': %w", namespace, pdbName, string(mergePatch), err)
			}
		}
	}
	return nil
}
