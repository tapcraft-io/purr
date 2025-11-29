package k8s

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tapcraft-io/purr/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Cache is the interface for Kubernetes resource caching
type Cache interface {
	Start(ctx context.Context) error
	Stop()
	IsReady() bool
	GetNamespaces() []string
	GetResourceByType(resourceType, namespace string) []types.ListItem

	// ClusterCache interface methods (for kubecomplete)
	Namespaces() []string
	ResourceTypes() []string
	ResourceTypesForCommand(path []string) []string
	ResourceNames(kind, namespace string) []string
	Containers(namespace, resourceKind, resourceName string) []string
}

// ResourceCache caches Kubernetes resources for quick access
type ResourceCache struct {
	clientset *kubernetes.Clientset

	// Cached resources
	namespaces   []corev1.Namespace
	pods         map[string][]corev1.Pod
	deployments  map[string][]appsv1.Deployment
	services     map[string][]corev1.Service
	configmaps   map[string][]corev1.ConfigMap
	secrets      map[string][]corev1.Secret
	ingresses    map[string][]networkingv1.Ingress
	statefulsets map[string][]appsv1.StatefulSet
	daemonsets   map[string][]appsv1.DaemonSet
	jobs         map[string][]batchv1.Job
	cronjobs     map[string][]batchv1.CronJob
	nodes        []corev1.Node

	// Metadata
	lastRefresh time.Time
	refreshing  atomic.Bool
	mu          sync.RWMutex

	// Context
	ctx    context.Context
	cancel context.CancelFunc
}

// NewResourceCache creates a new resource cache
func NewResourceCache(clientset *kubernetes.Clientset) *ResourceCache {
	return &ResourceCache{
		clientset:    clientset,
		pods:         make(map[string][]corev1.Pod),
		deployments:  make(map[string][]appsv1.Deployment),
		services:     make(map[string][]corev1.Service),
		configmaps:   make(map[string][]corev1.ConfigMap),
		secrets:      make(map[string][]corev1.Secret),
		ingresses:    make(map[string][]networkingv1.Ingress),
		statefulsets: make(map[string][]appsv1.StatefulSet),
		daemonsets:   make(map[string][]appsv1.DaemonSet),
		jobs:         make(map[string][]batchv1.Job),
		cronjobs:     make(map[string][]batchv1.CronJob),
	}
}

// Start initializes and starts background refresh
func (rc *ResourceCache) Start(ctx context.Context) error {
	rc.ctx, rc.cancel = context.WithCancel(ctx)

	// Initial refresh
	if err := rc.Refresh(); err != nil {
		return err
	}

	// Start background refresh (every 30 seconds)
	go rc.backgroundRefresh(30 * time.Second)

	return nil
}

// Stop stops the background refresh
func (rc *ResourceCache) Stop() {
	if rc.cancel != nil {
		rc.cancel()
	}
}

// Refresh updates all cached resources
func (rc *ResourceCache) Refresh() error {
	if !rc.refreshing.CompareAndSwap(false, true) {
		// Already refreshing
		return nil
	}
	defer rc.refreshing.Store(false)

	ctx := context.Background()
	if rc.ctx != nil {
		ctx = rc.ctx
	}

	// Refresh namespaces
	nsList, err := rc.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	rc.mu.Lock()
	rc.namespaces = nsList.Items
	rc.mu.Unlock()

	// Refresh resources for each namespace
	for _, ns := range nsList.Items {
		if err := rc.refreshNamespace(ctx, ns.Name); err != nil {
			// Log error but continue
			continue
		}
	}

	// Refresh cluster-wide resources
	nodesList, err := rc.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.nodes = nodesList.Items
		rc.mu.Unlock()
	}

	rc.mu.Lock()
	rc.lastRefresh = time.Now()
	rc.mu.Unlock()

	return nil
}

// refreshNamespace refreshes resources for a specific namespace
func (rc *ResourceCache) refreshNamespace(ctx context.Context, namespace string) error {
	// Pods
	podsList, err := rc.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.pods[namespace] = podsList.Items
		rc.mu.Unlock()
	}

	// Deployments
	depList, err := rc.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.deployments[namespace] = depList.Items
		rc.mu.Unlock()
	}

	// Services
	svcList, err := rc.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.services[namespace] = svcList.Items
		rc.mu.Unlock()
	}

	// ConfigMaps
	cmList, err := rc.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.configmaps[namespace] = cmList.Items
		rc.mu.Unlock()
	}

	// Secrets
	secretList, err := rc.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.secrets[namespace] = secretList.Items
		rc.mu.Unlock()
	}

	// StatefulSets
	stsList, err := rc.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.statefulsets[namespace] = stsList.Items
		rc.mu.Unlock()
	}

	// DaemonSets
	dsList, err := rc.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.daemonsets[namespace] = dsList.Items
		rc.mu.Unlock()
	}

	// Jobs
	jobsList, err := rc.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.jobs[namespace] = jobsList.Items
		rc.mu.Unlock()
	}

	// CronJobs
	cjList, err := rc.clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.cronjobs[namespace] = cjList.Items
		rc.mu.Unlock()
	}

	// Ingresses
	ingList, err := rc.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rc.mu.Lock()
		rc.ingresses[namespace] = ingList.Items
		rc.mu.Unlock()
	}

	return nil
}

// backgroundRefresh periodically refreshes the cache
func (rc *ResourceCache) backgroundRefresh(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rc.ctx.Done():
			return
		case <-ticker.C:
			_ = rc.Refresh()
		}
	}
}

// GetNamespaces returns all cached namespaces
func (rc *ResourceCache) GetNamespaces() []string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	names := make([]string, len(rc.namespaces))
	for i, ns := range rc.namespaces {
		names[i] = ns.Name
	}
	return names
}

// GetPods returns pods in a namespace
func (rc *ResourceCache) GetPods(namespace string) []corev1.Pod {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if pods, ok := rc.pods[namespace]; ok {
		result := make([]corev1.Pod, len(pods))
		copy(result, pods)
		return result
	}
	return []corev1.Pod{}
}

// GetDeployments returns deployments in a namespace
func (rc *ResourceCache) GetDeployments(namespace string) []appsv1.Deployment {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if deps, ok := rc.deployments[namespace]; ok {
		result := make([]appsv1.Deployment, len(deps))
		copy(result, deps)
		return result
	}
	return []appsv1.Deployment{}
}

// GetServices returns services in a namespace
func (rc *ResourceCache) GetServices(namespace string) []corev1.Service {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if svcs, ok := rc.services[namespace]; ok {
		result := make([]corev1.Service, len(svcs))
		copy(result, svcs)
		return result
	}
	return []corev1.Service{}
}

// GetNodes returns all nodes
func (rc *ResourceCache) GetNodes() []corev1.Node {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	result := make([]corev1.Node, len(rc.nodes))
	copy(result, rc.nodes)
	return result
}

// GetStatefulSets returns statefulsets in a namespace
func (rc *ResourceCache) GetStatefulSets(namespace string) []appsv1.StatefulSet {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if sts, ok := rc.statefulsets[namespace]; ok {
		result := make([]appsv1.StatefulSet, len(sts))
		copy(result, sts)
		return result
	}
	return []appsv1.StatefulSet{}
}

// GetDaemonSets returns daemonsets in a namespace
func (rc *ResourceCache) GetDaemonSets(namespace string) []appsv1.DaemonSet {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if ds, ok := rc.daemonsets[namespace]; ok {
		result := make([]appsv1.DaemonSet, len(ds))
		copy(result, ds)
		return result
	}
	return []appsv1.DaemonSet{}
}

// GetJobs returns jobs in a namespace
func (rc *ResourceCache) GetJobs(namespace string) []batchv1.Job {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if jobs, ok := rc.jobs[namespace]; ok {
		result := make([]batchv1.Job, len(jobs))
		copy(result, jobs)
		return result
	}
	return []batchv1.Job{}
}

// GetCronJobs returns cronjobs in a namespace
func (rc *ResourceCache) GetCronJobs(namespace string) []batchv1.CronJob {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if cj, ok := rc.cronjobs[namespace]; ok {
		result := make([]batchv1.CronJob, len(cj))
		copy(result, cj)
		return result
	}
	return []batchv1.CronJob{}
}

// GetConfigMaps returns configmaps in a namespace
func (rc *ResourceCache) GetConfigMaps(namespace string) []corev1.ConfigMap {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if cm, ok := rc.configmaps[namespace]; ok {
		result := make([]corev1.ConfigMap, len(cm))
		copy(result, cm)
		return result
	}
	return []corev1.ConfigMap{}
}

// GetSecrets returns secrets in a namespace
func (rc *ResourceCache) GetSecrets(namespace string) []corev1.Secret {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if secrets, ok := rc.secrets[namespace]; ok {
		result := make([]corev1.Secret, len(secrets))
		copy(result, secrets)
		return result
	}
	return []corev1.Secret{}
}

// GetIngresses returns ingresses in a namespace
func (rc *ResourceCache) GetIngresses(namespace string) []networkingv1.Ingress {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if ing, ok := rc.ingresses[namespace]; ok {
		result := make([]networkingv1.Ingress, len(ing))
		copy(result, ing)
		return result
	}
	return []networkingv1.Ingress{}
}

// GetResourceByType returns resources of a specific type
func (rc *ResourceCache) GetResourceByType(resourceType, namespace string) []types.ListItem {
	switch resourceType {
	case "pods", "pod", "po":
		return rc.PodsToListItems(rc.GetPods(namespace))
	case "deployments", "deployment", "deploy":
		return rc.DeploymentsToListItems(rc.GetDeployments(namespace))
	case "services", "service", "svc":
		return rc.ServicesToListItems(rc.GetServices(namespace))
	case "nodes", "node", "no":
		return rc.NodesToListItems(rc.GetNodes())
	case "namespaces", "namespace", "ns":
		return rc.NamespacesToListItems()
	case "statefulsets", "statefulset", "sts":
		return rc.StatefulSetsToListItems(rc.GetStatefulSets(namespace))
	case "daemonsets", "daemonset", "ds":
		return rc.DaemonSetsToListItems(rc.GetDaemonSets(namespace))
	case "jobs", "job":
		return rc.JobsToListItems(rc.GetJobs(namespace))
	case "cronjobs", "cronjob", "cj":
		return rc.CronJobsToListItems(rc.GetCronJobs(namespace))
	case "configmaps", "configmap", "cm":
		return rc.ConfigMapsToListItems(rc.GetConfigMaps(namespace))
	case "secrets", "secret":
		return rc.SecretsToListItems(rc.GetSecrets(namespace))
	case "ingresses", "ingress", "ing":
		return rc.IngressesToListItems(rc.GetIngresses(namespace))
	default:
		return []types.ListItem{}
	}
}

// PodsToListItems converts pods to list items
func (rc *ResourceCache) PodsToListItems(pods []corev1.Pod) []types.ListItem {
	items := make([]types.ListItem, len(pods))
	for i, pod := range pods {
		status := string(pod.Status.Phase)
		age := time.Since(pod.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       pod.Name,
			Description: fmt.Sprintf("Status: %s | Age: %s | NS: %s", status, age, pod.Namespace),
			Metadata: map[string]string{
				"namespace": pod.Namespace,
				"status":    status,
				"age":       age,
			},
		}
	}
	return items
}

// DeploymentsToListItems converts deployments to list items
func (rc *ResourceCache) DeploymentsToListItems(deps []appsv1.Deployment) []types.ListItem {
	items := make([]types.ListItem, len(deps))
	for i, dep := range deps {
		ready := fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, *dep.Spec.Replicas)
		age := time.Since(dep.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       dep.Name,
			Description: fmt.Sprintf("Ready: %s | Age: %s | NS: %s", ready, age, dep.Namespace),
			Metadata: map[string]string{
				"namespace": dep.Namespace,
				"ready":     ready,
				"age":       age,
			},
		}
	}
	return items
}

// ServicesToListItems converts services to list items
func (rc *ResourceCache) ServicesToListItems(svcs []corev1.Service) []types.ListItem {
	items := make([]types.ListItem, len(svcs))
	for i, svc := range svcs {
		svcType := string(svc.Spec.Type)
		age := time.Since(svc.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       svc.Name,
			Description: fmt.Sprintf("Type: %s | Age: %s | NS: %s", svcType, age, svc.Namespace),
			Metadata: map[string]string{
				"namespace": svc.Namespace,
				"type":      svcType,
				"age":       age,
			},
		}
	}
	return items
}

// NodesToListItems converts nodes to list items
func (rc *ResourceCache) NodesToListItems(nodes []corev1.Node) []types.ListItem {
	items := make([]types.ListItem, len(nodes))
	for i, node := range nodes {
		status := "Ready"
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
				status = "NotReady"
				break
			}
		}
		age := time.Since(node.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       node.Name,
			Description: fmt.Sprintf("Status: %s | Age: %s", status, age),
			Metadata: map[string]string{
				"status": status,
				"age":    age,
			},
		}
	}
	return items
}

// NamespacesToListItems converts namespaces to list items
func (rc *ResourceCache) NamespacesToListItems() []types.ListItem {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	items := make([]types.ListItem, len(rc.namespaces))
	for i, ns := range rc.namespaces {
		status := string(ns.Status.Phase)
		age := time.Since(ns.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       ns.Name,
			Description: fmt.Sprintf("Status: %s | Age: %s", status, age),
			Metadata: map[string]string{
				"status": status,
				"age":    age,
			},
		}
	}
	return items
}

// StatefulSetsToListItems converts statefulsets to list items
func (rc *ResourceCache) StatefulSetsToListItems(sts []appsv1.StatefulSet) []types.ListItem {
	items := make([]types.ListItem, len(sts))
	for i, s := range sts {
		ready := fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, *s.Spec.Replicas)
		age := time.Since(s.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       s.Name,
			Description: fmt.Sprintf("Ready: %s | Age: %s | NS: %s", ready, age, s.Namespace),
			Metadata: map[string]string{
				"namespace": s.Namespace,
				"ready":     ready,
				"age":       age,
			},
		}
	}
	return items
}

// DaemonSetsToListItems converts daemonsets to list items
func (rc *ResourceCache) DaemonSetsToListItems(ds []appsv1.DaemonSet) []types.ListItem {
	items := make([]types.ListItem, len(ds))
	for i, d := range ds {
		ready := fmt.Sprintf("%d/%d", d.Status.NumberReady, d.Status.DesiredNumberScheduled)
		age := time.Since(d.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       d.Name,
			Description: fmt.Sprintf("Ready: %s | Age: %s | NS: %s", ready, age, d.Namespace),
			Metadata: map[string]string{
				"namespace": d.Namespace,
				"ready":     ready,
				"age":       age,
			},
		}
	}
	return items
}

// JobsToListItems converts jobs to list items
func (rc *ResourceCache) JobsToListItems(jobs []batchv1.Job) []types.ListItem {
	items := make([]types.ListItem, len(jobs))
	for i, j := range jobs {
		status := fmt.Sprintf("%d/%d", j.Status.Succeeded, *j.Spec.Completions)
		age := time.Since(j.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       j.Name,
			Description: fmt.Sprintf("Succeeded: %s | Age: %s | NS: %s", status, age, j.Namespace),
			Metadata: map[string]string{
				"namespace": j.Namespace,
				"status":    status,
				"age":       age,
			},
		}
	}
	return items
}

// CronJobsToListItems converts cronjobs to list items
func (rc *ResourceCache) CronJobsToListItems(cj []batchv1.CronJob) []types.ListItem {
	items := make([]types.ListItem, len(cj))
	for i, c := range cj {
		schedule := c.Spec.Schedule
		age := time.Since(c.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       c.Name,
			Description: fmt.Sprintf("Schedule: %s | Age: %s | NS: %s", schedule, age, c.Namespace),
			Metadata: map[string]string{
				"namespace": c.Namespace,
				"schedule":  schedule,
				"age":       age,
			},
		}
	}
	return items
}

// ConfigMapsToListItems converts configmaps to list items
func (rc *ResourceCache) ConfigMapsToListItems(cm []corev1.ConfigMap) []types.ListItem {
	items := make([]types.ListItem, len(cm))
	for i, c := range cm {
		dataCount := fmt.Sprintf("%d keys", len(c.Data))
		age := time.Since(c.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       c.Name,
			Description: fmt.Sprintf("Data: %s | Age: %s | NS: %s", dataCount, age, c.Namespace),
			Metadata: map[string]string{
				"namespace": c.Namespace,
				"dataCount": dataCount,
				"age":       age,
			},
		}
	}
	return items
}

// SecretsToListItems converts secrets to list items
func (rc *ResourceCache) SecretsToListItems(secrets []corev1.Secret) []types.ListItem {
	items := make([]types.ListItem, len(secrets))
	for i, s := range secrets {
		dataCount := fmt.Sprintf("%d keys", len(s.Data))
		age := time.Since(s.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       s.Name,
			Description: fmt.Sprintf("Type: %s | Data: %s | Age: %s | NS: %s", s.Type, dataCount, age, s.Namespace),
			Metadata: map[string]string{
				"namespace": s.Namespace,
				"type":      string(s.Type),
				"dataCount": dataCount,
				"age":       age,
			},
		}
	}
	return items
}

// IngressesToListItems converts ingresses to list items
func (rc *ResourceCache) IngressesToListItems(ing []networkingv1.Ingress) []types.ListItem {
	items := make([]types.ListItem, len(ing))
	for i, ingress := range ing {
		var hosts []string
		for _, rule := range ingress.Spec.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
		}
		hostsStr := strings.Join(hosts, ",")
		age := time.Since(ingress.CreationTimestamp.Time).Round(time.Second).String()

		items[i] = types.ListItem{
			Title:       ingress.Name,
			Description: fmt.Sprintf("Hosts: %s | Age: %s | NS: %s", hostsStr, age, ingress.Namespace),
			Metadata: map[string]string{
				"namespace": ingress.Namespace,
				"hosts":     hostsStr,
				"age":       age,
			},
		}
	}
	return items
}

// IsReady returns true if the cache has been initialized
func (rc *ResourceCache) IsReady() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return !rc.lastRefresh.IsZero()
}

// Namespaces returns all cached namespace names (alias for GetNamespaces for ClusterCache interface)
func (rc *ResourceCache) Namespaces() []string {
	return rc.GetNamespaces()
}

// ResourceTypes returns all known resource types
func (rc *ResourceCache) ResourceTypes() []string {
	return []string{
		"pods", "po",
		"deployments", "deploy",
		"services", "svc",
		"replicasets", "rs",
		"statefulsets", "sts",
		"daemonsets", "ds",
		"jobs",
		"cronjobs", "cj",
		"configmaps", "cm",
		"secrets",
		"persistentvolumeclaims", "pvc",
		"persistentvolumes", "pv",
		"storageclasses", "sc",
		"ingresses", "ing",
		"networkpolicies", "netpol",
		"nodes", "no",
		"namespaces", "ns",
		"serviceaccounts", "sa",
		"roles",
		"rolebindings",
		"clusterroles",
		"clusterrolebindings",
		"replicationcontrollers", "rc",
		"horizontalpodautoscalers", "hpa",
		"poddisruptionbudgets", "pdb",
		"endpoints", "ep",
		"events", "ev",
		"limitranges", "limits",
		"resourcequotas", "quota",
	}
}

// ResourceTypesForCommand returns resource types specific to a command path
func (rc *ResourceCache) ResourceTypesForCommand(path []string) []string {
	if len(path) == 0 {
		return nil
	}

	// Special handling for certain commands
	key := strings.Join(path, " ")
	switch key {
	case "rollout restart", "rollout status", "rollout history", "rollout pause", "rollout resume", "rollout undo":
		return []string{"deployment", "deployments", "deploy", "daemonset", "daemonsets", "ds", "statefulset", "statefulsets", "sts"}
	case "logs":
		return []string{"pod", "pods", "po"}
	case "exec":
		return []string{"pod", "pods", "po"}
	case "top":
		return []string{"node", "nodes", "no", "pod", "pods", "po"}
	default:
		// Return all resource types for general commands
		return nil
	}
}

// ResourceNames returns names of resources of a given kind in a namespace
func (rc *ResourceCache) ResourceNames(kind, namespace string) []string {
	items := rc.GetResourceByType(kind, namespace)
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Title)
	}
	return names
}

// Containers returns container names for a given pod/workload
func (rc *ResourceCache) Containers(namespace, resourceKind, resourceName string) []string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if namespace == "" {
		namespace = "default"
	}

	var containers []string

	// Get containers from pods
	if pods, ok := rc.pods[namespace]; ok {
		for _, pod := range pods {
			// If resourceName is specified, only get containers from that pod
			if resourceName != "" && pod.Name != resourceName {
				continue
			}

			for _, container := range pod.Spec.Containers {
				containers = append(containers, container.Name)
			}
			for _, container := range pod.Spec.InitContainers {
				containers = append(containers, container.Name)
			}
		}
	}

	// If looking for a deployment/statefulset/daemonset, find their pods and get containers
	if resourceName != "" && (resourceKind == "deployment" || resourceKind == "deployments" || resourceKind == "deploy") {
		if deps, ok := rc.deployments[namespace]; ok {
			for _, dep := range deps {
				if dep.Name == resourceName {
					for _, container := range dep.Spec.Template.Spec.Containers {
						containers = append(containers, container.Name)
					}
					for _, container := range dep.Spec.Template.Spec.InitContainers {
						containers = append(containers, container.Name)
					}
				}
			}
		}
	}

	if resourceName != "" && (resourceKind == "statefulset" || resourceKind == "statefulsets" || resourceKind == "sts") {
		if sts, ok := rc.statefulsets[namespace]; ok {
			for _, s := range sts {
				if s.Name == resourceName {
					for _, container := range s.Spec.Template.Spec.Containers {
						containers = append(containers, container.Name)
					}
					for _, container := range s.Spec.Template.Spec.InitContainers {
						containers = append(containers, container.Name)
					}
				}
			}
		}
	}

	if resourceName != "" && (resourceKind == "daemonset" || resourceKind == "daemonsets" || resourceKind == "ds") {
		if ds, ok := rc.daemonsets[namespace]; ok {
			for _, d := range ds {
				if d.Name == resourceName {
					for _, container := range d.Spec.Template.Spec.Containers {
						containers = append(containers, container.Name)
					}
					for _, container := range d.Spec.Template.Spec.InitContainers {
						containers = append(containers, container.Name)
					}
				}
			}
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	unique := make([]string, 0, len(containers))
	for _, c := range containers {
		if !seen[c] {
			seen[c] = true
			unique = append(unique, c)
		}
	}

	return unique
}
