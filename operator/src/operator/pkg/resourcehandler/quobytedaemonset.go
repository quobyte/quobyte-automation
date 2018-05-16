package resourcehandler

import (
	"encoding/json"
	"fmt"

	"github.com/golang/glog"
	"k8s.io/api/apps/v1beta2"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func updateVersionLabel(dep *extensionsv1beta1.DaemonSet, version string) {
	labels := dep.GetLabels()
	podLabels := dep.Spec.Template.ObjectMeta.GetLabels()
	labels["version"] = version
	podLabels["version"] = version
	dep.SetLabels(labels)
	dep.Spec.Template.ObjectMeta.SetLabels(podLabels)
	glog.Infof("Updated labels to %s", version)
}

//DeleteQuobyteDeployment deletes Quobyte daemonset with the given name.
func DeleteQuobyteDeployment(name string) error {
	err = KubernetesClient.ExtensionsV1beta1().DaemonSets(quobyteNameSpace).Delete(name, &metav1.DeleteOptions{})

	if err != nil {
		return err
	}

	return nil
}

func GetDaemonsetByName(name string) (*extensionsv1beta1.DaemonSet, error) {
	return KubernetesClient.ExtensionsV1beta1().DaemonSets(quobyteNameSpace).Get(name, metav1.GetOptions{})
}

// UpdateDaemonSet updates daemonset, with ondelete rolling update strategy and given version.
func UpdateDaemonSet(daemonsetname string, image string) error {

	ds, err := GetDaemonsetByName(daemonsetname)
	if err != nil {
		fmt.Printf("Unable to read client daemonset: %v\n", err)
		return err
	}

	oldData, err := json.Marshal(ds)
	if err != nil {
		return err
	}

	// version := GetVersionFromString(image)
	//updateVersionLabel(ds, version)
	ds.Spec.UpdateStrategy.Type = "OnDelete" // TODO: move it to the definition in yaml to make it default
	ds.Spec.Template.Spec.Containers[0].Image = image

	newJSON, err := json.Marshal(ds)
	patchbytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newJSON, v1beta2.DaemonSet{})

	if len(patchbytes) > 2 {
		updatedDS, err := KubernetesClient.ExtensionsV1beta1().DaemonSets(quobyteNameSpace).Patch(ds.Name, types.StrategicMergePatchType, patchbytes)
		if err != nil {
			glog.Errorf("update of client daemonset failed: %v", err)
			return err
		}
		glog.Infof("updated daemonset %s", updatedDS.Name)
	}
	return nil
}
