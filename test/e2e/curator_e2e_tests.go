package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/operate-first/curator-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: global variable for controller-runtime's dynamic client
// - Create this `c` in the e2e_suite_test.go

// Stick with using When/Describe "containers" instead of top-level It statements
// because you can clean up resources that were created within an individual test
// using BeforeEach/AfterEach "container" logic.

var _ = Describe("fetchdata controller", func() {
	When("a new FetchData custom resource has been created", func() {
		var (
			fd  *v1alpha1.FetchData
			ctx context.Context
		)
		BeforeEach(func() {
			ctx = context.Background()

			fd = &v1alpha1.FetchData{
				ObjectMeta: v1.ObjectMeta{
					GenerateName: "stub",
				},
				Spec: v1alpha1.FetchDataSpec{
					Schedule: "* 0 * * *",
					// TODO: fill out the rest of the spec
				},
			}

			Expect(c.Create(ctx, fd)).To(BeNil())
		})
		AfterEach(func() {
			Expect(c.Delete(ctx, fd)).To(BeNil())
		})
		It("should create a CronJob resource", func() {
			Eventually(func() error {
				cronJobs := &batchv1.CronJobList{}
				// List the CronJobs in the fd.Spec.CronjobNamespace namespace
				if err := c.List(ctx, cronJobs, client.InNamespace(fd.Spec.CronjobNamespace)); err != nil {
					return err
				}
				for _, cronJob := range cronJobs.Items {
					// TODO: ensure that the FetchData resource successfully generate a CronJob resource
					// TODO: ensure that the generated CronJob resource matches our spec configuration
				}
			}).Should(Succeed())
		})
	})
})

/*
High-level steps:

Download the ginkgo v2 CLI locally

Update the CI GHA workflow and run unit + e2e tests in separate jobs

Update the Makefile and change the test target to `test-unit`

Update the Makefile and introduce a test-e2e target:
- That e2e target runs `ginkgo -trace -progress test/e2e`

Introduce ginkgo as a tools.go dependency

Download the https://github.com/kubernetes-sigs/kind dependency

Create a kind cluster in CI

Steps to scaffold out a barebones e2e suite using ginkgo:
- Create the test/e2e directory
- Run `ginkgo bootstrap` on the test/e2e directory to generate an e2e_suite_test.go file
- Create a test/e2e/curator_e2e_tests.go file that uses the `e2e` package and contains a single
  top-level Describe container.
- We can introduce tests for individual components or top-level functionality within that Describe container.
*/
