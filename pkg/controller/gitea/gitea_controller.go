package gitea

import (
	"context"
	"log"

	integreatlyv1alpha1 "github.com/integr8ly/gitea-operator/pkg/apis/integreatly/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Add creates a new Gitea Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileGitea{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("gitea-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Gitea
	err = c.Watch(&source.Kind{Type: &integreatlyv1alpha1.Gitea{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Gitea
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &integreatlyv1alpha1.Gitea{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileGitea{}

// ReconcileGitea reconciles a Gitea object
type ReconcileGitea struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Gitea object and makes changes based on the state read
// and what is in the Gitea.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileGitea) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("Reconciling Gitea %s/%s\n", request.Namespace, request.Name)

	// Fetch the Gitea instance
	instance := &integreatlyv1alpha1.Gitea{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	instanceCopy := instance.DeepCopy()

	// Try create all gitea resources
	r.CreateResource(instanceCopy, GiteaServiceAccountName)
	r.CreateResource(instanceCopy, GiteaPgServiceName)
	r.CreateResource(instanceCopy, GiteaPgDeploymentName)
	r.CreateResource(instanceCopy, GiteaServiceName)
	r.CreateResource(instanceCopy, GiteaDeploymentName)
	r.CreateResource(instanceCopy, GiteaReposPvcName)
	r.CreateResource(instanceCopy, GiteaPgPvcName)
	r.CreateResource(instanceCopy, GiteaConfigMapName)

	// The oauth-proxy is only compatible with Openshift because it
	// does not support ingress
	if instanceCopy.Spec.DeployProxy {
		r.CreateResource(instanceCopy, ProxyServiceAccountName)
		r.CreateResource(instanceCopy, ProxyServiceName)
		r.CreateResource(instanceCopy, ProxyDeploymentName)
		r.CreateResource(instanceCopy, ProxyRouteName)
	} else {
		r.CreateResource(instanceCopy, GiteaIngressName)
	}

	return reconcile.Result{}, nil
}

// Creates a generic kubernetes resource from a template
func (r *ReconcileGitea) CreateResource(cr *integreatlyv1alpha1.Gitea, resourceName string) {
	resourceHelper := newResourceHelper(cr)
	resource, err := resourceHelper.createResource(resourceName)

	if err != nil {
		log.Printf("Error parsing template: %s", err)
		return
	}

	// Try to find the resource, it may already exist
	selector := types.NamespacedName{
		Namespace: cr.Namespace,
		Name: resourceName,
	}
	err = r.client.Get(context.TODO(), selector, resource)

	// The resource exists, do nothing
	if err == nil {
		return
	}

	// Resource does not exist or something went wrong
	if errors.IsNotFound(err) {
		log.Printf("Resource '%s' is missing. Creating it.", resourceName)
	} else {
		log.Printf("Error reading resource '%s': %s", resourceName, err)
		return
	}

	// Set the CR as the owner of this resource so that when
	// the CR is deleted this resource also gets removed
	controllerutil.SetControllerReference(cr, resource.(v1.Object), r.scheme)

	err = r.client.Create(context.TODO(), resource)
	if err !=  nil {
		log.Printf("Error creating resource: %s", err)
	}
}
