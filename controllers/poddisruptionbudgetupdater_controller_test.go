package controllers

import (
	"context"
	"fmt"
	poddisruptionbudgetupdaterv1alpha1 "pod-disruption-budget-updater/api/v1alpha1"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	eventuallyTimeout  = time.Second * 10
	eventuallyInterval = time.Millisecond * 250
	validateWait       = time.Millisecond * 400
)

func getIntOrStrInt(val int) *intstr.IntOrString {
	intOrStr := intstr.FromInt(val)
	return &intOrStr
}

func getIntOrStrStr(val string) *intstr.IntOrString {
	intOrStr := intstr.FromString(val)
	return &intOrStr
}

func getTimeString(ts time.Time) string {
	hours, minutes, seconds := ts.Clock()
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func deployPodDisruptionBudget(namespace, name string, minAvailable, maxUnavailable *intstr.IntOrString) {
	budgetManaged1 := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable:   minAvailable,
			MaxUnavailable: maxUnavailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"deployment": name},
			},
		},
	}
	Expect(k8sClient.Create(ctx, budgetManaged1)).Should(Succeed())
}

var _ = Describe("PodDisruptionBudgetUpdater controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		crName = "poddisruptionbudgetupdater-operator"
		nsBase = "ns"
	)
	var (
		podDisruptionBudgets = []string{}
		specs                = []poddisruptionbudgetupdaterv1alpha1.PodDisruptionBudgetUpdaterSpec{}
		namespace            = ""
	)
	BeforeEach(func() {
		podDisruptionBudgets = []string{"pdb-managed-1", "pdb-managed-2", "pdb-noexist-1"}
		startTime := time.Now()
		fromTime1, toTime1 := startTime.Add(time.Second*15), startTime.Add(time.Second*20)
		fromTime2, toTime2 := startTime.Add(time.Second*25), startTime.Add(time.Second*30)
		defaultSpec := poddisruptionbudgetupdaterv1alpha1.DefaultSpec{
			Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrInt(0)},
		}
		rulesSpec := []poddisruptionbudgetupdaterv1alpha1.RulesSpec{
			{
				Periods:  []poddisruptionbudgetupdaterv1alpha1.PeriodSpec{{From: getTimeString(fromTime1), To: getTimeString(toTime1)}},
				Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			},
			{
				Periods:  []poddisruptionbudgetupdaterv1alpha1.PeriodSpec{{From: getTimeString(fromTime2), To: getTimeString(toTime2)}},
				Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrStr("50%"), MaxUnavailable: nil},
			},
		}
		namespace = fmt.Sprintf("%s-%s", nsBase, randString(10))
		_ = specs
		ctx := context.Background()

		By("By creating a containing namespace")
		nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(k8sClient.Create(ctx, nsSpec)).Should(Succeed())

		By("Register a PodDisruptionBudgetUpdater CRD")
		crd1 := &poddisruptionbudgetupdaterv1alpha1.PodDisruptionBudgetUpdater{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "poddisruptionbudgetupdater.k8s.faith/v1alpha1",
				Kind:       "PodDisruptionBudgetUpdater",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      crName,
				Namespace: namespace,
			},
			Spec: poddisruptionbudgetupdaterv1alpha1.PodDisruptionBudgetUpdaterSpec{
				PodDisruptionBudgets: podDisruptionBudgets,
				Default:              defaultSpec,
				Rules:                rulesSpec,
			},
		}
		Expect(k8sClient.Create(ctx, crd1)).Should(Succeed())

		By("Creating PodDisruptionBudgets for this controller to manage")
		deployPodDisruptionBudget(namespace, "pdb-managed-1", getIntOrStrInt(0), nil)
		deployPodDisruptionBudget(namespace, "pdb-managed-2", getIntOrStrInt(1), nil)
		deployPodDisruptionBudget(namespace, "pdb-managed-3", getIntOrStrInt(2), nil)
		deployPodDisruptionBudget(namespace, "pdb-unmanaged-1", getIntOrStrInt(1), nil)
	})
	/* Not possible as finalizers won't run
	AfterEach(func() {
		By("Deleting the namespace")
		Expect(k8sClient.Delete(ctx, r)).To(Succeed())
	})
	*/
	Context("Testing the controller", func() {
		It("Should update the PodDisruptionBudgets whith default values", func() {
			By("Applying controller default values in the beginning")
			validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
				"pdb-managed-1":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
				"pdb-managed-2":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
				"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
				"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			})
			By("Patching the CRD so that the fefault MinAvailable is 5")
			updateCRD(namespace, crName, nil,
				&poddisruptionbudgetupdaterv1alpha1.DefaultSpec{Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrInt(5)}},
				nil)
			validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
				"pdb-managed-1":   {MinAvailable: getIntOrStrInt(5), MaxUnavailable: nil},
				"pdb-managed-2":   {MinAvailable: getIntOrStrInt(5), MaxUnavailable: nil},
				"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
				"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			})
			By("Patching the CRD so that it also manages pdb-managed-3")
			updateCRD(namespace, crName, []string{"pdb-managed-1", "pdb-managed-2", "pdb-managed-3", "pdb-noexist-2"}, nil, nil)
			validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
				"pdb-managed-1":   {MinAvailable: getIntOrStrInt(5), MaxUnavailable: nil},
				"pdb-managed-2":   {MinAvailable: getIntOrStrInt(5), MaxUnavailable: nil},
				"pdb-managed-3":   {MinAvailable: getIntOrStrInt(5), MaxUnavailable: nil},
				"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			})
			By("Patching the CRD so that it no longer manages pdb-managed-1 and has 6 as defailt MinAvailable")
			updateCRD(namespace, crName, []string{"pdb-managed-2", "pdb-managed-3", "pdb-noexist-2"},
				&poddisruptionbudgetupdaterv1alpha1.DefaultSpec{Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrInt(6)}},
				nil)
			validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
				"pdb-managed-1":   {MinAvailable: getIntOrStrInt(5), MaxUnavailable: nil},
				"pdb-managed-2":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-managed-3":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			})
			By("Patching the CRD so that it only manages pdb-managed-1 anagain the non-existent pdb-managed-4")
			updateCRD(namespace, crName, []string{"pdb-managed-1", "pdb-managed-4", "pdb-noexist-2"}, nil, nil)
			validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
				"pdb-managed-1":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-managed-2":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-managed-3":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			})
			By("Patching the CRD so that it has 7 as defailt MinAvailable")
			updateCRD(namespace, crName, nil,
				&poddisruptionbudgetupdaterv1alpha1.DefaultSpec{Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrInt(7)}},
				nil)
			validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
				"pdb-managed-1":   {MinAvailable: getIntOrStrInt(7), MaxUnavailable: nil},
				"pdb-managed-2":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-managed-3":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			})
			By("Creating pdb-managed-4 for the Operator to manage")
			deployPodDisruptionBudget(namespace, "pdb-managed-4", getIntOrStrInt(3), nil)
			validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
				"pdb-managed-1":   {MinAvailable: getIntOrStrInt(7), MaxUnavailable: nil},
				"pdb-managed-2":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-managed-3":   {MinAvailable: getIntOrStrInt(6), MaxUnavailable: nil},
				"pdb-managed-4":   {MinAvailable: getIntOrStrInt(7), MaxUnavailable: nil},
				"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
			})
		})
		It("Should update the PodDisruptionBudgets whith time based values", func() {
			By("Applying controller default values in the beginning", func() {
				validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
					"pdb-managed-1":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
					"pdb-managed-2":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
					"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
					"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
				})
			})
			fromTime1, toTime1 := time.Now().Add(validateWait*4), time.Now().Add(validateWait*8)
			fromTime2, toTime2 := time.Now().Add(validateWait*12), time.Now().Add(validateWait*16)
			By("Patching the controllers so that alternative values are applied for a short period in the very near future", func() {
				updateCRD(namespace, crName, nil, nil, []poddisruptionbudgetupdaterv1alpha1.RulesSpec{
					{
						Periods:  []poddisruptionbudgetupdaterv1alpha1.PeriodSpec{{From: getTimeString(fromTime1), To: getTimeString(toTime1)}},
						Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrInt(3), MaxUnavailable: nil},
					},
					{
						Periods:  []poddisruptionbudgetupdaterv1alpha1.PeriodSpec{{From: getTimeString(fromTime2), To: getTimeString(toTime2)}},
						Settings: poddisruptionbudgetupdaterv1alpha1.SettingsSpec{MinAvailable: getIntOrStrStr("50%"), MaxUnavailable: nil},
					},
				})
				By("Validating that default values are still used as we are outside the period with alternative settings", func() {
					validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
						"pdb-managed-1":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
						"pdb-managed-2":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
						"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
						"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
					})
				})
				By("Validating that alternative settings are used when in the first period with alternative settings", func() {
					for time.Now().Before(fromTime1) {
						time.Sleep(validateWait / 4)
					}
					validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
						"pdb-managed-1":   {MinAvailable: getIntOrStrInt(3), MaxUnavailable: nil},
						"pdb-managed-2":   {MinAvailable: getIntOrStrInt(3), MaxUnavailable: nil},
						"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
						"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
					})
				})
				By("Validating that default settings are used between periods with alternative settings", func() {
					for time.Now().Before(toTime1) {
						time.Sleep(validateWait / 4)
					}
					validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
						"pdb-managed-1":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
						"pdb-managed-2":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
						"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
						"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
					})
				})
				By("Validating that alternative settings are used in second period with alternative settings", func() {
					for time.Now().Before(fromTime2) {
						time.Sleep(validateWait / 4)
					}
					validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
						"pdb-managed-1":   {MinAvailable: getIntOrStrStr("50%"), MaxUnavailable: nil},
						"pdb-managed-2":   {MinAvailable: getIntOrStrStr("50%"), MaxUnavailable: nil},
						"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
						"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
					})
				})
				By("Validating that default settings are used after periods with alternative settings", func() {
					for time.Now().Before(toTime2) {
						time.Sleep(validateWait / 4)
					}
					validatePodDisruptionBudgets(namespace, map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec{
						"pdb-managed-1":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
						"pdb-managed-2":   {MinAvailable: getIntOrStrInt(0), MaxUnavailable: nil},
						"pdb-managed-3":   {MinAvailable: getIntOrStrInt(2), MaxUnavailable: nil},
						"pdb-unmanaged-1": {MinAvailable: getIntOrStrInt(1), MaxUnavailable: nil},
					})
				})
			})
		})
	})
})

func updateCRD(namespace, name string,
	podDisruptionBudgets []string,
	defaultSpec *poddisruptionbudgetupdaterv1alpha1.DefaultSpec,
	rules []poddisruptionbudgetupdaterv1alpha1.RulesSpec) {
	crd := &poddisruptionbudgetupdaterv1alpha1.PodDisruptionBudgetUpdater{}
	Eventually(func() error {
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, crd)
		if err != nil {
			return err
		}
		if podDisruptionBudgets != nil {
			crd.Spec.PodDisruptionBudgets = podDisruptionBudgets
		}
		if defaultSpec != nil {
			crd.Spec.Default = *defaultSpec
		}
		if rules != nil {
			crd.Spec.Rules = rules
		}
		return k8sClient.Update(context.TODO(), crd)
	}, eventuallyTimeout, eventuallyInterval).Should(Succeed())
}

func validatePodDisruptionBudgets(namespace string, expected map[string]*poddisruptionbudgetupdaterv1alpha1.SettingsSpec) {
	time.Sleep(validateWait)
	for budgetName, expect := range expected {
		budget := &policyv1.PodDisruptionBudget{}
		Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: budgetName}, budget)).Should(Succeed())
		if budget.Spec.MinAvailable != nil || expect.MinAvailable != nil {
			if budget.Spec.MinAvailable == nil {
				Expect(fmt.Errorf("PodDisruptionBudget %s had MinAvailable = nil, but it should have been %s", budgetName, expect.MinAvailable.String())).Should(Succeed())
			} else if expect.MinAvailable == nil {
				Expect(fmt.Errorf("PodDisruptionBudget %s had MinAvailable = %s, but it should have been nil", budgetName, budget.Spec.MinAvailable.String())).Should(Succeed())
			} else if expect.MinAvailable.String() != budget.Spec.MinAvailable.String() {
				Expect(fmt.Errorf("PodDisruptionBudget %s had MinAvailable = %s, but it should have been %s", budgetName, budget.Spec.MinAvailable.String(), expect.MinAvailable.String())).Should(Succeed())
			}
		}
		if budget.Spec.MaxUnavailable != nil || expect.MaxUnavailable != nil {
			if budget.Spec.MaxUnavailable == nil {
				Expect(fmt.Errorf("PodDisruptionBudget %s had MaxUnavailable = nil, but it should have been %s", budgetName, expect.MaxUnavailable.String())).Should(Succeed())
			} else if expect.MaxUnavailable == nil {
				Expect(fmt.Errorf("PodDisruptionBudget %s had MaxUnavailable = %s, but it should have been nil", budgetName, budget.Spec.MinAvailable.String())).Should(Succeed())
			} else if expect.MaxUnavailable.String() != budget.Spec.MaxUnavailable.String() {
				Expect(fmt.Errorf("PodDisruptionBudget %s had MaxUnavailable = %s, but it should have been %s", budgetName, budget.Spec.MinAvailable.String(), expect.MaxUnavailable.String())).Should(Succeed())
			}
		}
	}
}
