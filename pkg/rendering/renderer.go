// Copyright Contributors to the Open Cluster Management project
package renderer

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	loader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/fatih/structs"
	v1 "github.com/stolostron/backplane-operator/api/v1"
	"github.com/stolostron/backplane-operator/pkg/utils"
	"helm.sh/helm/v3/pkg/engine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

const (
	AlwaysChartsDir = "pkg/templates/charts/always"
)

type Values struct {
	Global    Global    `yaml:"global" structs:"global"`
	HubConfig HubConfig `yaml:"hubconfig" structs:"hubconfig"`
	Org       string    `yaml:"org" structs:"org"`
}

type Global struct {
	ImageOverrides map[string]string `yaml:"imageOverrides" structs:"imageOverrides"`
	PullPolicy     string            `yaml:"pullPolicy" structs:"pullPolicy"`
	PullSecret     string            `yaml:"pullSecret" structs:"pullSecret"`
	Namespace      string            `yaml:"namespace" structs:"namespace"`
}

type HubConfig struct {
	NodeSelector map[string]string   `yaml:"nodeSelector" structs:"nodeSelector"`
	ProxyConfigs map[string]string   `yaml:"proxyConfigs" structs:"proxyConfigs"`
	ReplicaCount int                 `yaml:"replicaCount" structs:"replicaCount"`
	Tolerations  []corev1.Toleration `yaml:"tolerations" structs:"tolerations"`
}

func RenderCRDs(crdDir string) ([]*unstructured.Unstructured, []error) {
	var crds []*unstructured.Unstructured
	errs := []error{}

	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		crdDir = path.Join(val, crdDir)
	}

	// Read CRD files
	err := filepath.Walk(crdDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err.Error())
			return err
		}
		crd := &unstructured.Unstructured{}
		if info == nil || info.IsDir() {
			return nil
		}
		bytesFile, e := ioutil.ReadFile(path)
		if e != nil {
			errs = append(errs, fmt.Errorf("%s - error reading file: %v", info.Name(), err.Error()))
		}
		if err = yaml.Unmarshal(bytesFile, crd); err != nil {
			errs = append(errs, fmt.Errorf("%s - error unmarshalling file to unstructured: %v", info.Name(), err.Error()))
		}
		crds = append(crds, crd)
		return nil
	})
	if err != nil {
		return crds, errs
	}

	return crds, errs
}

func RenderCharts(chartDir string, backplaneConfig *v1.MultiClusterEngine, images map[string]string) ([]*unstructured.Unstructured, []error) {
	log := log.FromContext(context.Background())
	var templates []*unstructured.Unstructured
	errs := []error{}
	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		chartDir = path.Join(val, chartDir)
	}
	charts, err := ioutil.ReadDir(chartDir)
	if err != nil {
		errs = append(errs, err)
	}
	for _, chart := range charts {
		chartPath := filepath.Join(chartDir, chart.Name())
		chartTemplates, errs := renderTemplates(chartPath, backplaneConfig, images)
		if len(errs) > 0 {
			for _, err := range errs {
				log.Info(err.Error())
			}
			return nil, errs
		}
		templates = append(templates, chartTemplates...)
	}
	return templates, nil
}

func RenderChart(chartPath string, backplaneConfig *v1.MultiClusterEngine, images map[string]string) ([]*unstructured.Unstructured, []error) {
	log := log.FromContext(context.Background())
	errs := []error{}
	if val, ok := os.LookupEnv("DIRECTORY_OVERRIDE"); ok {
		chartPath = path.Join(val, chartPath)
	}
	chartTemplates, errs := renderTemplates(chartPath, backplaneConfig, images)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Info(err.Error())
		}
		return nil, errs
	}
	return chartTemplates, nil

}

func renderTemplates(chartPath string, backplaneConfig *v1.MultiClusterEngine, images map[string]string) ([]*unstructured.Unstructured, []error) {
	log := log.FromContext(context.Background())
	var templates []*unstructured.Unstructured
	errs := []error{}
	chart, err := loader.Load(chartPath)
	if err != nil {
		log.Info(fmt.Sprintf("error loading chart: %s", chart.Name()))
		return nil, append(errs, err)
	}
	valuesYaml := &Values{}
	injectValuesOverrides(valuesYaml, backplaneConfig, images)
	helmEngine := engine.Engine{
		Strict:   true,
		LintMode: false,
	}
	rawTemplates, err := helmEngine.Render(chart, chartutil.Values{"Values": structs.Map(valuesYaml)})
	if err != nil {
		log.Info(fmt.Sprintf("error rendering chart: %s", chart.Name()))
		return nil, append(errs, err)
	}

	for fileName, templateFile := range rawTemplates {
		unstructured := &unstructured.Unstructured{}
		if err = yaml.Unmarshal([]byte(templateFile), unstructured); err != nil {
			return nil, append(errs, fmt.Errorf("error converting file %s to unstructured", fileName))
		}

		utils.AddBackplaneConfigLabels(unstructured, backplaneConfig.Name)

		// Add namespace to namespaced resources
		switch unstructured.GetKind() {
		case "Deployment", "ServiceAccount", "Role", "RoleBinding", "Service", "ConfigMap":
			unstructured.SetNamespace(backplaneConfig.Spec.TargetNamespace)
		}
		templates = append(templates, unstructured)
	}

	return templates, errs
}

func injectValuesOverrides(values *Values, backplaneConfig *v1.MultiClusterEngine, images map[string]string) {

	values.Global.ImageOverrides = images

	values.Global.PullPolicy = string(utils.GetImagePullPolicy(backplaneConfig))

	values.Global.Namespace = backplaneConfig.Spec.TargetNamespace

	values.Global.PullSecret = backplaneConfig.Spec.ImagePullSecret

	values.HubConfig.ReplicaCount = utils.DefaultReplicaCount(backplaneConfig)

	values.HubConfig.NodeSelector = backplaneConfig.Spec.NodeSelector

	if len(backplaneConfig.Spec.Tolerations) > 0 {
		values.HubConfig.Tolerations = backplaneConfig.Spec.Tolerations
	} else {
		values.HubConfig.Tolerations = utils.DefaultTolerations()
	}

	values.Org = "open-cluster-management"

	if utils.ProxyEnvVarsAreSet() {
		proxyVar := map[string]string{}
		proxyVar["HTTP_PROXY"] = os.Getenv("HTTP_PROXY")
		proxyVar["HTTPS_PROXY"] = os.Getenv("HTTPS_PROXY")
		proxyVar["NO_PROXY"] = os.Getenv("NO_PROXY")
		values.HubConfig.ProxyConfigs = proxyVar
	}

	// TODO: Define all overrides
}
