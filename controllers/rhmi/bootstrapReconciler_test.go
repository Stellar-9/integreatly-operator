package controllers

import (
	"context"
	"errors"
	"fmt"
	integreatlyv1alpha1 "github.com/integr8ly/integreatly-operator/apis/v1alpha1"
	moqclient "github.com/integr8ly/integreatly-operator/pkg/client"
	"github.com/integr8ly/integreatly-operator/pkg/config"
	l "github.com/integr8ly/integreatly-operator/pkg/resources/logger"
	"github.com/integr8ly/integreatly-operator/pkg/resources/marketplace"
	userHelper "github.com/integr8ly/integreatly-operator/pkg/resources/user"
	prometheusv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

var (
	rhoamOperatorNs         = "redhat-rhoam-operator"
	threescaleNs            = "redhat-rhoam-3scale"
	threescaleOperatorNs    = "redhat-rhoam-3scale-operator"
	croNs                   = "redhat-rhoam-cloud-resources"
	customerMonitoringNs    = "redhat-rhoam-customer-monitoring"
	marin3rNs               = "redhat-rhoam-marin3r"
	marin3rOperatorNs       = "redhat-rhoam-marin3r-operator"
	monitoringNs            = "redhat-rhoam-monitoring"
	observabilityNs         = "redhat-rhoam-observability"
	observabilityOperatorNs = "redhat-rhoam-observability-operator"
	rhssoNs                 = "redhat-rhoam-rhsso"
	rhssoOperatorNs         = "redhat-rhoam-rhsso-operator"
	userSsoNs               = "redhat-rhoam-user-sso"
	userSsoOperatorNs       = "redhat-rhoam-user-sso-operator"
	someRandomNs            = "some-random-nspace"
)

func TestReconciler_reconcileRHMIConfigPermissions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = rbacv1.SchemeBuilder.AddToScheme(scheme)

	tests := []struct {
		Name           string
		ExpectedStatus integreatlyv1alpha1.StatusPhase
		FakeConfig     *config.ConfigReadWriterMock
		FakeMPM        *marketplace.MarketplaceInterfaceMock
		Installation   *integreatlyv1alpha1.RHMI
		Recorder       record.EventRecorder
		FakeClient     k8sclient.Client
		Assertion      func(k8sclient.Client) error
	}{
		{
			Name: "Test Role and Role Binding is not created",
			FakeConfig: &config.ConfigReadWriterMock{
				GetOperatorNamespaceFunc: func() string {
					return "test-namespace"
				},
			},
			FakeMPM:        &marketplace.MarketplaceInterfaceMock{},
			Installation:   &integreatlyv1alpha1.RHMI{},
			Recorder:       record.NewFakeRecorder(50),
			ExpectedStatus: integreatlyv1alpha1.PhaseCompleted,
			FakeClient:     fakeclient.NewFakeClientWithScheme(scheme),
			Assertion:      assertRoleBindingNotFound,
		},
		{
			Name: "Test - error in creating role and role binding",
			FakeConfig: &config.ConfigReadWriterMock{
				GetOperatorNamespaceFunc: func() string {
					return "test-namespace"
				},
			},
			FakeMPM:        &marketplace.MarketplaceInterfaceMock{},
			Installation:   &integreatlyv1alpha1.RHMI{},
			Recorder:       record.NewFakeRecorder(50),
			ExpectedStatus: integreatlyv1alpha1.PhaseFailed,
			FakeClient: &moqclient.SigsClientInterfaceMock{
				GetFunc: func(ctx context.Context, key types.NamespacedName, obj runtime.Object) error {
					return errors.New("dummy get error")
				},
				CreateFunc: func(ctx context.Context, obj runtime.Object, opts ...k8sclient.CreateOption) error {
					return errors.New("dummy create error")
				},
			},
			Assertion: func(client k8sclient.Client) error {
				return nil
			},
		},
		{
			Name: "Test that existing role binding is deleted",
			FakeConfig: &config.ConfigReadWriterMock{
				GetOperatorNamespaceFunc: func() string {
					return "test-namespace"
				},
			},
			FakeMPM:        &marketplace.MarketplaceInterfaceMock{},
			Installation:   &integreatlyv1alpha1.RHMI{},
			Recorder:       record.NewFakeRecorder(50),
			ExpectedStatus: integreatlyv1alpha1.PhaseCompleted,
			FakeClient: fake.NewFakeClientWithScheme(scheme, &rbacv1.RoleBinding{
				ObjectMeta: v1.ObjectMeta{
					Name:      "rhmiconfig-dedicated-admins-role-binding",
					Namespace: "test-namespace",
				},
			}),
			Assertion: assertRoleBindingNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			reconciler, err := NewBootstrapReconciler(tt.FakeConfig, tt.Installation, tt.FakeMPM, tt.Recorder, l.NewLogger())
			if err != nil {
				t.Fatalf("Error creating bootstrap reconciler: %s", err)
			}

			phase, err := reconciler.reconcileRHMIConfigPermissions(context.TODO(), tt.FakeClient)

			if phase != tt.ExpectedStatus {
				t.Fatalf("Expected %s phase but got %s", tt.ExpectedStatus, phase)
			}

			if err := tt.Assertion(tt.FakeClient); err != nil {
				t.Fatalf("Failed assertion: %v", err)
			}

		})
	}
}

func TestReconciler_reconcilePrometheusRules(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = prometheusv1.SchemeBuilder.AddToScheme(scheme)
	_ = corev1.SchemeBuilder.AddToScheme(scheme)

	tests := []struct {
		Name           string
		ExpectedStatus integreatlyv1alpha1.StatusPhase
		FakeConfig     *config.ConfigReadWriterMock
		FakeMPM        *marketplace.MarketplaceInterfaceMock
		Installation   *integreatlyv1alpha1.RHMI
		Recorder       record.EventRecorder
		FakeClient     k8sclient.Client
		Assertion      func(k8sclient.Client) error
	}{
		{
			Name: "Test that all exisiting prometheus rules in given ns are removed correctly",
			FakeConfig: &config.ConfigReadWriterMock{
				GetOperatorNamespaceFunc: func() string {
					return rhoamOperatorNs
				},
			},
			FakeMPM: &marketplace.MarketplaceInterfaceMock{},
			Installation: &integreatlyv1alpha1.RHMI{
				ObjectMeta: v1.ObjectMeta{
					Namespace: rhoamOperatorNs,
				},
				Spec: integreatlyv1alpha1.RHMISpec{
					NamespacePrefix: "redhat-rhoam-",
				},
			},
			Recorder:       record.NewFakeRecorder(50),
			ExpectedStatus: integreatlyv1alpha1.PhaseCompleted,
			FakeClient: fake.NewFakeClientWithScheme(scheme,
				&prometheusv1.PrometheusRuleList{
					Items: getPrometheusRules(),
				},
				getNamespaces(),
			),
			Assertion: assertPrometheusRulesDeletion,
		},
		{
			Name: "Test that prometheus rules in other ns are NOT removed",
			FakeConfig: &config.ConfigReadWriterMock{
				GetOperatorNamespaceFunc: func() string {
					return rhoamOperatorNs
				},
			},
			FakeMPM: &marketplace.MarketplaceInterfaceMock{},
			Installation: &integreatlyv1alpha1.RHMI{
				ObjectMeta: v1.ObjectMeta{
					Namespace: rhoamOperatorNs,
				},
				Spec: integreatlyv1alpha1.RHMISpec{
					NamespacePrefix: "redhat-rhoam-",
				},
			},
			Recorder:       record.NewFakeRecorder(50),
			ExpectedStatus: integreatlyv1alpha1.PhaseCompleted,
			FakeClient: fake.NewFakeClientWithScheme(scheme,
				&prometheusv1.PrometheusRuleList{
					Items: getPrometheusRules(),
				},
				getNamespaces(),
			),
			Assertion: assertPrometheusRulesNoDeletion,
		},
		{
			Name: "Test that all expected namespaces are returned",
			FakeConfig: &config.ConfigReadWriterMock{
				GetOperatorNamespaceFunc: func() string {
					return rhoamOperatorNs
				},
			},
			FakeMPM: &marketplace.MarketplaceInterfaceMock{},
			Installation: &integreatlyv1alpha1.RHMI{
				ObjectMeta: v1.ObjectMeta{
					Namespace: rhoamOperatorNs,
				},
				Spec: integreatlyv1alpha1.RHMISpec{
					NamespacePrefix: "redhat-rhoam-",
				},
			},
			Recorder:       record.NewFakeRecorder(50),
			ExpectedStatus: integreatlyv1alpha1.PhaseCompleted,
			FakeClient: fake.NewFakeClientWithScheme(scheme,
				&prometheusv1.PrometheusRuleList{
					Items: getPrometheusRules(),
				},
				getNamespaces(),
			),
			Assertion: assertAllExpectedNamespacesAreReturned,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			reconciler, err := NewBootstrapReconciler(tt.FakeConfig, tt.Installation, tt.FakeMPM, tt.Recorder, l.NewLogger())
			if err != nil {
				t.Fatalf("Error creating bootstrap reconciler: %s", err)
			}

			phase, err := reconciler.removePrometheusRules(context.TODO(), tt.FakeClient, "redhat-rhoam-")

			if phase != tt.ExpectedStatus {
				t.Fatalf("Expected %s phase but got %s", tt.ExpectedStatus, phase)
			}

			if err := tt.Assertion(tt.FakeClient); err != nil {
				t.Fatalf("Failed assertion: %v", err)
			}
		})
	}
}

func assertRoleBindingNotFound(client k8sclient.Client) error {
	configRole := &rbacv1.Role{}
	err := client.Get(context.TODO(), k8sclient.ObjectKey{
		Name:      "rhmiconfig-dedicated-admins-role",
		Namespace: "test-namespace",
	}, configRole)
	if err == nil {
		return errors.New("Role rhmiconfig-dedicated-admins-role should not exist")
	}

	if !k8serr.IsNotFound(err) {
		return fmt.Errorf("Unexpected error occurred: %v", err)
	}

	return nil
}

func assertAllExpectedNamespacesAreReturned(client k8sclient.Client) error {
	existingNamespaces, err := getRHOAMNamespaces(context.TODO(), client, "redhat-rhoam-")
	if err != nil {
		return err
	} else if existingNamespaces == nil {
		return fmt.Errorf("No namespaces were found")
	}
	rhoamFound := false
	threescaleFound := false
	threescaleOpFound := false
	croFound := false
	customerMonitoringFound := false
	marin3rFound := false
	marin3rOperatorFound := false
	monitoringNsFound := false
	observabilityNsFound := false
	observabilityOperatorFound := false
	rhssoFound := false
	rhssoOperatorFound := false
	userSSOFound := false
	userSSOOperatorFound := false
	randomNsFound := false

	for _, namespaceFound := range existingNamespaces {
		if namespaceFound == rhoamOperatorNs {
			rhoamFound = true
		}
		if namespaceFound == threescaleNs {
			threescaleFound = true
		}
		if namespaceFound == threescaleOperatorNs {
			threescaleOpFound = true
		}
		if namespaceFound == croNs {
			croFound = true
		}
		if namespaceFound == customerMonitoringNs {
			customerMonitoringFound = true
		}
		if namespaceFound == marin3rNs {
			marin3rFound = true
		}
		if namespaceFound == marin3rOperatorNs {
			marin3rOperatorFound = true
		}
		if namespaceFound == monitoringNs {
			monitoringNsFound = true
		}
		if namespaceFound == observabilityNs {
			observabilityNsFound = true
		}
		if namespaceFound == observabilityOperatorNs {
			observabilityOperatorFound = true
		}
		if namespaceFound == rhssoNs {
			rhssoFound = true
		}
		if namespaceFound == rhssoOperatorNs {
			rhssoOperatorFound = true
		}
		if namespaceFound == userSsoNs {
			userSSOFound = true
		}
		if namespaceFound == userSsoOperatorNs {
			userSSOOperatorFound = true
		}
		if namespaceFound == someRandomNs {
			randomNsFound = true
		}

	}

	if !rhoamFound || !croFound || !threescaleFound || !threescaleOpFound || !customerMonitoringFound || !marin3rFound || !marin3rOperatorFound || !monitoringNsFound ||
		!rhssoFound || !rhssoOperatorFound || !userSSOFound || !userSSOOperatorFound {
		return fmt.Errorf("Not all namespaces were found")
	}

	if observabilityNsFound || observabilityOperatorFound || randomNsFound {
		return fmt.Errorf("observability namespace was found while it should have been skipped")
	}

	return nil
}

func assertPrometheusRulesDeletion(client k8sclient.Client) error {
	var allExistingRules []prometheusv1.PrometheusRule
	rhoamProductNamespaces, err := getRHOAMNamespaces(context.TODO(), client, "redhat-rhoam-")
	if err != nil {
		return err
	}
	for _, namespace := range rhoamProductNamespaces {
		namespaceRules := &prometheusv1.PrometheusRuleList{}

		err := client.List(context.TODO(), namespaceRules, k8sclient.InNamespace(namespace))
		if err != nil {
			return err
		} else if k8serr.IsNotFound(err) || len(namespaceRules.Items) == 0 {
			continue
		}

		for _, rule := range namespaceRules.Items {
			if rule.Name != "keycloak" {
				allExistingRules = append(allExistingRules, *rule)
			}
		}
	}
	if len(allExistingRules) != 0 {
		return fmt.Errorf("Found prometheus rules that should have been deleted")
	}

	return nil
}

func assertPrometheusRulesNoDeletion(client k8sclient.Client) error {
	existingRules := &prometheusv1.PrometheusRuleList{}

	err := client.List(context.TODO(), existingRules, k8sclient.InNamespace(observabilityNs))
	if err != nil {
		return err
	} else if len(existingRules.Items) == 0 {
		return fmt.Errorf("Other ns prometheus rules were also removed while they should not")
	}
	err = client.List(context.TODO(), existingRules, k8sclient.InNamespace(observabilityOperatorNs))
	if err != nil {
		return err
	} else if len(existingRules.Items) == 0 {
		return fmt.Errorf("Other ns prometheus rules were also removed while they should not")
	}
	return nil
}

func getPrometheusRules() []*prometheusv1.PrometheusRule {
	return []*prometheusv1.PrometheusRule{
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule1",
				Namespace: rhoamOperatorNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule2",
				Namespace: threescaleNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule3",
				Namespace: threescaleOperatorNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule4",
				Namespace: croNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule5",
				Namespace: customerMonitoringNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule6",
				Namespace: marin3rNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule7",
				Namespace: marin3rOperatorNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule8",
				Namespace: monitoringNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule9",
				Namespace: observabilityNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule10",
				Namespace: observabilityOperatorNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule11",
				Namespace: rhoamOperatorNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule12",
				Namespace: rhssoNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule13",
				Namespace: userSsoNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testrule14",
				Namespace: userSsoOperatorNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "keycloak",
				Namespace: rhssoNs,
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:      "keycloak",
				Namespace: userSsoNs,
			},
		},
	}
}

func getNamespaces() *corev1.NamespaceList {
	return &corev1.NamespaceList{
		TypeMeta: v1.TypeMeta{},
		ListMeta: v1.ListMeta{},
		Items: []corev1.Namespace{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: rhoamOperatorNs,
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: threescaleNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: threescaleOperatorNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: croNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: customerMonitoringNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: marin3rNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: marin3rOperatorNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: monitoringNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: observabilityNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: observabilityOperatorNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: rhssoNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: rhssoOperatorNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: userSsoNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: userSsoOperatorNs,
					Labels: map[string]string{
						"integreatly": "true",
					},
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: someRandomNs,
				},
			},
		},
	}
}

func Test_tenantExists(t *testing.T) {
	type args struct {
		user    string
		tenants []userHelper.MultiTenantUser
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Empty list of tenants given",
			args: args{
				user:    "username",
				tenants: []userHelper.MultiTenantUser{},
			},
			want: false,
		},
		{
			name: "Tenant list is nil",
			args: args{
				user:    "username",
				tenants: nil,
			},
			want: false,
		},
		{
			name: "Name not in tenant list given",
			args: args{
				user: "tenantName",
				tenants: []userHelper.MultiTenantUser{
					{
						TenantName: "tenantName01",
					},
					{
						TenantName: "tenantName02",
					},
				},
			},
			want: false,
		},
		{
			name: "Name in list of tenants, list length 1",
			args: args{
				user: "tenantName",
				tenants: []userHelper.MultiTenantUser{
					{
						TenantName: "tenantName",
					},
				},
			},
			want: true,
		},
		{
			name: "Name in list of tenants, list length 2",
			args: args{
				user: "tenantName",
				tenants: []userHelper.MultiTenantUser{
					{
						TenantName: "tenantName01",
					},
					{
						TenantName: "tenantName",
					},
				},
			},
			want: true,
		},
		{
			name: "Tenant name is empty string",
			args: args{
				user: "",
				tenants: []userHelper.MultiTenantUser{
					{
						TenantName: "tenantName01",
					},
					{
						TenantName: "tenantName02",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tenantExists(tt.args.user, tt.args.tenants); got != tt.want {
				t.Errorf("tenantExists() = %v, want %v", got, tt.want)
			}
		})
	}
}
