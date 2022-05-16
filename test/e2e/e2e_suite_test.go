package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/operate-first/curator-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	c client.Client
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

var _ = BeforeSuite(func() {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	Expect(err).To(BeNil())

	err = batchv1.AddToScheme(scheme)
	Expect(err).To(BeNil())

	c, err = client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	Expect(err).To(BeNil())
})
