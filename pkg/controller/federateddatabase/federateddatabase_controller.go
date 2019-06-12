package federateddatabase

import (
	"context"
	"reflect"
	sysdesengv1alpha1 "github.com/mvazquezc/federated-db/pkg/apis/sysdeseng/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	appsv1 "k8s.io/api/apps/v1"
	fedv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/core/v1alpha1"
)

var log = logf.Log.WithName("controller_federateddatabase")

// Add creates a new FederatedDatabase Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	// Add the federation API types to the schema
	if err := fedv1a1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		return err
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileFederatedDatabase{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("federateddatabase-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource FederatedDatabase
	err = c.Watch(&source.Kind{Type: &sysdesengv1alpha1.FederatedDatabase{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployments and requeue the owner FederatedDatabase

	// In the future we should create federatedDeployment instead of Deployments
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &sysdesengv1alpha1.FederatedDatabase{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileFederatedDatabase{}

// ReconcileFederatedDatabase reconciles a FederatedDatabase object
type ReconcileFederatedDatabase struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a FederatedDatabase object and makes changes based on the state read
// and what is in the FederatedDatabase.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileFederatedDatabase) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling FederatedDatabase")

	// Fetch the FederatedDatabase instance
	instance := &sysdesengv1alpha1.FederatedDatabase{}
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

	// In the future we will use FederatedDeployments
	// Get a deployment for our database
	deployment := newDeploymentForCR(instance)
	found := &appsv1.Deployment{}

	// Set FederatedDatabase instance as the owner and controller of the deployment
	if err := controllerutil.SetControllerReference(instance, deployment, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Get configured replicas and deployment size from the Spec
	specReplicas := instance.Spec.Replicas
	specSize := instance.Spec.DeploymentSize

	// Get FederatedClusters list
	clusterList := &fedv1a1.FederatedClusterList{}
	listOpts := &client.ListOptions{
		Namespace: instance.Namespace,
	}
	err = r.client.List(context.TODO(), listOpts, clusterList)
	if err != nil {
		reqLogger.Error(err, "Failed to list clusters.", "FederatedDatabase.Namespace", instance.Namespace, "FederatedDatabase.Name", instance.Name)
		return reconcile.Result{}, err
	}

	// Get the FederatedClusters names
	clusterNames := getClusterNames(clusterList.Items)
	// Get the length of the array containing the FederatedClusters names
	clusterListLength := len(clusterNames)

	// Check if desired replicas are higher than available clusters
	// If true, we are not going to reconcile the object as we cannot place all replicas
	if (specReplicas > int32(clusterListLength)) {
		log.Info("More replicas configured than clusters available")
		return reconcile.Result{}, nil
	}

	// Check if this Deployment already exists
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// The Deployment does not exists, create it
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		err = r.client.Create(context.TODO(), deployment)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Deployment created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// We have size which determines the number of pods, and replicas that determines the propagation size
	// Ensure deployment replicas and placement match the desired state
	if *found.Spec.Replicas != specSize {
		log.Info("Current deployment replicas do not match FederatedDatabase configured DeploymentSize")
		found.Spec.Replicas = &specSize
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment.", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return reconcile.Result{}, err
		}
		// Spec updated - return and requeue (so we can update status)
		return reconcile.Result{Requeue: true}, nil
	}
	// The code below is just for reference, it won't configure placements
	//placementLength := len(*found.Spec.Placement)
	placementLength := 0

	if int32(placementLength) != specReplicas {
		// Here we should get the cluster list and configure the FederatedDeployment placement
		// if we had three clusters and we wanted two replicas, we should select two clusters from the list
		var deploymentPlacements []string
		for i := 0; i < int(specReplicas); i++ {
			deploymentPlacements = append(deploymentPlacements,clusterNames[i])
		}
		log.Info("Deployment Information", "Replicas Configured", specReplicas, "Pods per Replica", specSize, "Available Clusters", clusterListLength, "Possible Placements", deploymentPlacements)
		// Here we will update the FederatedDeployment object
		// Spec updated - return and requeue (so we can update status)
		//return reconcile.Result{Requeue: true}, nil
	}

	// Update status for FederatedDatabase object
	// Set the default status
	federatedDatabaseStatus := "NotSet"
	if *found.Spec.Replicas == specSize {
		federatedDatabaseStatus = "Deployment Size OK, Replicas Placement KO"
		if int32(placementLength) == specReplicas {
			federatedDatabaseStatus = "Deployment Size OK, Replicas Placement OK"
		}
	} else {
		federatedDatabaseStatus = "KO"
	}
	// If the current status is not the configured status, update it
	if !reflect.DeepEqual(federatedDatabaseStatus, instance.Status.Health) {
		instance.Status.Health = federatedDatabaseStatus
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update FederatedDatabase status.")
			return reconcile.Result{}, err
		}
	}

	// Deployment already exists - don't requeue
	reqLogger.Info("Skip reconcile: Deployment already exists", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
	return reconcile.Result{}, nil
}

// Returns a new deployment without replicas configured (when federated, placement should be empty as well)
// replicas and placement will be configured in the sync loop
func newDeploymentForCR(cr *sysdesengv1alpha1.FederatedDatabase) *appsv1.Deployment {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-" + cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "busybox",
						Name:  "busybox",
						Command: []string{"sleep", "3600"},
					}},
				},
			},
		},
	}
}

// getClusterNames returns the FederatedCluster names
func getClusterNames(clusters []fedv1a1.FederatedCluster) []string {
	var clusterNames []string
	for _, cluster := range clusters {
		clusterNames = append(clusterNames, cluster.Name)
	}
	return clusterNames
}
