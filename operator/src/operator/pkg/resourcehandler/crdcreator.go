package resourcehandler

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/golang/glog"

	"github.com/ghodss/yaml"
	v1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

//CreateAllQuobyteCrd Creates all the required quobyte custom resource definitions under /artifacts with -crd.yaml.
func CreateAllQuobyteCrd() error {
	glog.Info("Creating all quobyte CRD")
	files, err := ioutil.ReadDir("pkg/artifacts")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		matched, err := filepath.Match("*-crd.yaml", file.Name())
		if err != nil {
			return err
		}
		if matched {
			def, err := ioutil.ReadFile(fmt.Sprintf("pkg/artifacts/%s", file.Name()))
			if err != nil {
				glog.Errorf("Unalbe to get definition of crd %s", file.Name())
				return err
			}
			crd := &v1beta1.CustomResourceDefinition{}
			err = yaml.Unmarshal(def, crd)

			if err != nil {
				glog.Errorf("Failed to get the CRD %s definition\n", file.Name())
				return err
			}
			_, err = APIServerClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					glog.Infof("CRD %s already exists\n", file.Name())
					continue
				}
				glog.Errorf("Failed to create CRD %s\n", file.Name())
				return err
			}
		}
	}
	return nil
}
