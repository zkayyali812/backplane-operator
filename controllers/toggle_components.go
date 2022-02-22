package controllers

import (
	"context"
	"fmt"

	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	"github.com/stolostron/backplane-operator/pkg/foundation"
	renderer "github.com/stolostron/backplane-operator/pkg/rendering"
	"github.com/stolostron/backplane-operator/pkg/toggle"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *MultiClusterEngineReconciler) ensureConsoleMCE(ctx context.Context, backplaneConfig *backplanev1.MultiClusterEngine) (ctrl.Result, error) {
	namespacedName := types.NamespacedName{Name: "console-mce", Namespace: backplaneConfig.Spec.TargetNamespace}
	r.StatusManager.RemoveComponent(toggle.DisabledStatus(namespacedName, []*unstructured.Unstructured{}))
	r.StatusManager.AddComponent(toggle.EnabledStatus(namespacedName))

	log := log.FromContext(ctx)

	templates, errs := renderer.RenderChart(toggle.ConsoleMCEChartsDir, backplaneConfig, r.Images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
	}

	// Applies all templates
	for _, template := range templates {
		result, err := r.applyTemplate(ctx, backplaneConfig, template)
		if err != nil {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterEngineReconciler) ensureNoConsoleMCE(ctx context.Context, backplaneConfig *backplanev1.MultiClusterEngine) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	namespacedName := types.NamespacedName{Name: "console-mce", Namespace: backplaneConfig.Spec.TargetNamespace}

	// Renders all templates from charts
	templates, errs := renderer.RenderChart(toggle.ConsoleMCEChartsDir, backplaneConfig, r.Images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
	}

	r.StatusManager.RemoveComponent(toggle.EnabledStatus(namespacedName))
	r.StatusManager.AddComponent(toggle.DisabledStatus(namespacedName, []*unstructured.Unstructured{}))

	// Deletes all templates
	for _, template := range templates {
		result, err := r.deleteTemplate(ctx, backplaneConfig, template)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to delete Console MCE template: %s", template.GetName()))
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterEngineReconciler) ensureManagedServiceAccount(ctx context.Context, backplaneConfig *backplanev1.MultiClusterEngine) (ctrl.Result, error) {
	r.StatusManager.RemoveComponent(toggle.DisabledStatus(types.NamespacedName{Name: "managedservice", Namespace: backplaneConfig.Spec.TargetNamespace}, []*unstructured.Unstructured{}))
	r.StatusManager.AddComponent(toggle.EnabledStatus(types.NamespacedName{Name: "managed-serviceaccount-addon-manager", Namespace: backplaneConfig.Spec.TargetNamespace}))

	log := log.FromContext(ctx)

	if foundation.CanInstallAddons(ctx, r.Client) {
		// Render CRD templates
		crdPath := toggle.ManagedServiceAccountCRDPath
		crds, errs := renderer.RenderCRDs(crdPath)
		if len(errs) > 0 {
			for _, err := range errs {
				log.Info(err.Error())
			}
			return ctrl.Result{RequeueAfter: requeuePeriod}, nil
		}

		// Apply all CRDs
		for _, crd := range crds {
			result, err := r.applyTemplate(ctx, backplaneConfig, crd)
			if err != nil {
				return result, err
			}
		}

		// Renders all templates from charts
		chartPath := toggle.ManagedServiceAccountChartDir
		templates, errs := renderer.RenderChart(chartPath, backplaneConfig, r.Images)
		if len(errs) > 0 {
			for _, err := range errs {
				log.Info(err.Error())
			}
			return ctrl.Result{RequeueAfter: requeuePeriod}, nil
		}

		// Applies all templates
		for _, template := range templates {
			result, err := r.applyTemplate(ctx, backplaneConfig, template)
			if err != nil {
				return result, err
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterEngineReconciler) ensureNoManagedServiceAccount(ctx context.Context, backplaneConfig *backplanev1.MultiClusterEngine) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Renders all templates from charts
	chartPath := toggle.ManagedServiceAccountChartDir
	templates, errs := renderer.RenderChart(chartPath, backplaneConfig, r.Images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
	}

	r.StatusManager.RemoveComponent(toggle.EnabledStatus(types.NamespacedName{Name: "managed-serviceaccount-addon-manager", Namespace: backplaneConfig.Spec.TargetNamespace}))
	r.StatusManager.AddComponent(toggle.DisabledStatus(types.NamespacedName{Name: "managedservice", Namespace: backplaneConfig.Spec.TargetNamespace}, []*unstructured.Unstructured{}))

	// Deletes all templates
	for _, template := range templates {
		if template.GetKind() == foundation.ClusterManagementAddonKind && !foundation.CanInstallAddons(ctx, r.Client) {
			// Can't delete ClusterManagementAddon if Kind doesn't exists
			continue
		}
		result, err := r.deleteTemplate(ctx, backplaneConfig, template)
		if err != nil {
			log.Error(err, "Failed to delete MSA template")
			return result, err
		}
	}

	// Render CRD templates
	crdPath := toggle.ManagedServiceAccountCRDPath
	crds, errs := renderer.RenderCRDs(crdPath)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return ctrl.Result{RequeueAfter: requeuePeriod}, nil
	}

	// Delete all CRDs
	for _, crd := range crds {
		result, err := r.deleteTemplate(ctx, backplaneConfig, crd)
		if err != nil {
			log.Error(err, "Failed to delete CRD")
			return result, err
		}
	}
	return ctrl.Result{}, nil
}
