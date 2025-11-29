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

// Start initializes and starts background refresh with watchers
func (rc *ResourceCache) Start(ctx context.Context) error {
	rc.ctx, rc.cancel = context.WithCancel(ctx)

	// Initial refresh
	if err := rc.Refresh(); err != nil {
		return err
	}

	// Start watchers for real-time updates
	go rc.watchNamespaces()
	go rc.watchPods()
	go rc.watchDeployments()
	go rc.watchServices()
	go rc.watchNodes()
	go rc.watchConfigMaps()
	go rc.watchSecrets()
	go rc.watchStatefulSets()
	go rc.watchDaemonSets()
	go rc.watchJobs()
	go rc.watchCronJobs()
	go rc.watchIngresses()

	// Still do periodic full refresh as a fallback (every 5 minutes)
	// This catches any missed events and handles reconnections
	go rc.backgroundRefresh(5 * time.Minute)

	return nil
}

// Stop stops the background refresh
func (rc *ResourceCache) Stop() {
	if rc.cancel != nil {
		rc.cancel()
	}
}

// watchNamespaces watches for namespace changes and updates cache
func (rc *ResourceCache) watchNamespaces() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.CoreV1().Namespaces().Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			ns, ok := event.Object.(*corev1.Namespace)
			if !ok {
				continue
			}

			rc.mu.Lock()
			switch event.Type {
			case "ADDED":
				// Check if already exists
				exists := false
				for _, existing := range rc.namespaces {
					if existing.Name == ns.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.namespaces = append(rc.namespaces, *ns)
				}
			case "DELETED":
				for i, existing := range rc.namespaces {
					if existing.Name == ns.Name {
						rc.namespaces = append(rc.namespaces[:i], rc.namespaces[i+1:]...)
						// Clean up associated resources
						delete(rc.pods, ns.Name)
						delete(rc.deployments, ns.Name)
						delete(rc.services, ns.Name)
						delete(rc.configmaps, ns.Name)
						delete(rc.secrets, ns.Name)
						delete(rc.statefulsets, ns.Name)
						delete(rc.daemonsets, ns.Name)
						delete(rc.jobs, ns.Name)
						delete(rc.cronjobs, ns.Name)
						delete(rc.ingresses, ns.Name)
						break
					}
				}
			case "MODIFIED":
				for i, existing := range rc.namespaces {
					if existing.Name == ns.Name {
						rc.namespaces[i] = *ns
						break
					}
				}
			}
			rc.mu.Unlock()
		}

		// Watcher closed, restart after brief delay
		time.Sleep(time.Second)
	}
}

// watchPods watches for pod changes across all namespaces
func (rc *ResourceCache) watchPods() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.CoreV1().Pods("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := pod.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.pods[ns]; !ok {
					rc.pods[ns] = []corev1.Pod{}
				}
				// Check if already exists
				exists := false
				for _, existing := range rc.pods[ns] {
					if existing.Name == pod.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.pods[ns] = append(rc.pods[ns], *pod)
				}
			case "DELETED":
				if pods, ok := rc.pods[ns]; ok {
					for i, existing := range pods {
						if existing.Name == pod.Name {
							rc.pods[ns] = append(pods[:i], pods[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if pods, ok := rc.pods[ns]; ok {
					for i, existing := range pods {
						if existing.Name == pod.Name {
							rc.pods[ns][i] = *pod
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchDeployments watches for deployment changes across all namespaces
func (rc *ResourceCache) watchDeployments() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.AppsV1().Deployments("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			dep, ok := event.Object.(*appsv1.Deployment)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := dep.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.deployments[ns]; !ok {
					rc.deployments[ns] = []appsv1.Deployment{}
				}
				exists := false
				for _, existing := range rc.deployments[ns] {
					if existing.Name == dep.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.deployments[ns] = append(rc.deployments[ns], *dep)
				}
			case "DELETED":
				if deps, ok := rc.deployments[ns]; ok {
					for i, existing := range deps {
						if existing.Name == dep.Name {
							rc.deployments[ns] = append(deps[:i], deps[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if deps, ok := rc.deployments[ns]; ok {
					for i, existing := range deps {
						if existing.Name == dep.Name {
							rc.deployments[ns][i] = *dep
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchServices watches for service changes across all namespaces
func (rc *ResourceCache) watchServices() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.CoreV1().Services("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			svc, ok := event.Object.(*corev1.Service)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := svc.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.services[ns]; !ok {
					rc.services[ns] = []corev1.Service{}
				}
				exists := false
				for _, existing := range rc.services[ns] {
					if existing.Name == svc.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.services[ns] = append(rc.services[ns], *svc)
				}
			case "DELETED":
				if svcs, ok := rc.services[ns]; ok {
					for i, existing := range svcs {
						if existing.Name == svc.Name {
							rc.services[ns] = append(svcs[:i], svcs[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if svcs, ok := rc.services[ns]; ok {
					for i, existing := range svcs {
						if existing.Name == svc.Name {
							rc.services[ns][i] = *svc
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchNodes watches for node changes
func (rc *ResourceCache) watchNodes() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.CoreV1().Nodes().Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			node, ok := event.Object.(*corev1.Node)
			if !ok {
				continue
			}

			rc.mu.Lock()
			switch event.Type {
			case "ADDED":
				exists := false
				for _, existing := range rc.nodes {
					if existing.Name == node.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.nodes = append(rc.nodes, *node)
				}
			case "DELETED":
				for i, existing := range rc.nodes {
					if existing.Name == node.Name {
						rc.nodes = append(rc.nodes[:i], rc.nodes[i+1:]...)
						break
					}
				}
			case "MODIFIED":
				for i, existing := range rc.nodes {
					if existing.Name == node.Name {
						rc.nodes[i] = *node
						break
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchConfigMaps watches for configmap changes across all namespaces
func (rc *ResourceCache) watchConfigMaps() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.CoreV1().ConfigMaps("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			cm, ok := event.Object.(*corev1.ConfigMap)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := cm.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.configmaps[ns]; !ok {
					rc.configmaps[ns] = []corev1.ConfigMap{}
				}
				exists := false
				for _, existing := range rc.configmaps[ns] {
					if existing.Name == cm.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.configmaps[ns] = append(rc.configmaps[ns], *cm)
				}
			case "DELETED":
				if cms, ok := rc.configmaps[ns]; ok {
					for i, existing := range cms {
						if existing.Name == cm.Name {
							rc.configmaps[ns] = append(cms[:i], cms[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if cms, ok := rc.configmaps[ns]; ok {
					for i, existing := range cms {
						if existing.Name == cm.Name {
							rc.configmaps[ns][i] = *cm
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchSecrets watches for secret changes across all namespaces
func (rc *ResourceCache) watchSecrets() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.CoreV1().Secrets("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			secret, ok := event.Object.(*corev1.Secret)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := secret.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.secrets[ns]; !ok {
					rc.secrets[ns] = []corev1.Secret{}
				}
				exists := false
				for _, existing := range rc.secrets[ns] {
					if existing.Name == secret.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.secrets[ns] = append(rc.secrets[ns], *secret)
				}
			case "DELETED":
				if secrets, ok := rc.secrets[ns]; ok {
					for i, existing := range secrets {
						if existing.Name == secret.Name {
							rc.secrets[ns] = append(secrets[:i], secrets[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if secrets, ok := rc.secrets[ns]; ok {
					for i, existing := range secrets {
						if existing.Name == secret.Name {
							rc.secrets[ns][i] = *secret
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchStatefulSets watches for statefulset changes across all namespaces
func (rc *ResourceCache) watchStatefulSets() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.AppsV1().StatefulSets("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			sts, ok := event.Object.(*appsv1.StatefulSet)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := sts.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.statefulsets[ns]; !ok {
					rc.statefulsets[ns] = []appsv1.StatefulSet{}
				}
				exists := false
				for _, existing := range rc.statefulsets[ns] {
					if existing.Name == sts.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.statefulsets[ns] = append(rc.statefulsets[ns], *sts)
				}
			case "DELETED":
				if stsList, ok := rc.statefulsets[ns]; ok {
					for i, existing := range stsList {
						if existing.Name == sts.Name {
							rc.statefulsets[ns] = append(stsList[:i], stsList[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if stsList, ok := rc.statefulsets[ns]; ok {
					for i, existing := range stsList {
						if existing.Name == sts.Name {
							rc.statefulsets[ns][i] = *sts
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchDaemonSets watches for daemonset changes across all namespaces
func (rc *ResourceCache) watchDaemonSets() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.AppsV1().DaemonSets("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			ds, ok := event.Object.(*appsv1.DaemonSet)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := ds.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.daemonsets[ns]; !ok {
					rc.daemonsets[ns] = []appsv1.DaemonSet{}
				}
				exists := false
				for _, existing := range rc.daemonsets[ns] {
					if existing.Name == ds.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.daemonsets[ns] = append(rc.daemonsets[ns], *ds)
				}
			case "DELETED":
				if dsList, ok := rc.daemonsets[ns]; ok {
					for i, existing := range dsList {
						if existing.Name == ds.Name {
							rc.daemonsets[ns] = append(dsList[:i], dsList[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if dsList, ok := rc.daemonsets[ns]; ok {
					for i, existing := range dsList {
						if existing.Name == ds.Name {
							rc.daemonsets[ns][i] = *ds
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchJobs watches for job changes across all namespaces
func (rc *ResourceCache) watchJobs() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.BatchV1().Jobs("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := job.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.jobs[ns]; !ok {
					rc.jobs[ns] = []batchv1.Job{}
				}
				exists := false
				for _, existing := range rc.jobs[ns] {
					if existing.Name == job.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.jobs[ns] = append(rc.jobs[ns], *job)
				}
			case "DELETED":
				if jobs, ok := rc.jobs[ns]; ok {
					for i, existing := range jobs {
						if existing.Name == job.Name {
							rc.jobs[ns] = append(jobs[:i], jobs[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if jobs, ok := rc.jobs[ns]; ok {
					for i, existing := range jobs {
						if existing.Name == job.Name {
							rc.jobs[ns][i] = *job
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchCronJobs watches for cronjob changes across all namespaces
func (rc *ResourceCache) watchCronJobs() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.BatchV1().CronJobs("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			cj, ok := event.Object.(*batchv1.CronJob)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := cj.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.cronjobs[ns]; !ok {
					rc.cronjobs[ns] = []batchv1.CronJob{}
				}
				exists := false
				for _, existing := range rc.cronjobs[ns] {
					if existing.Name == cj.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.cronjobs[ns] = append(rc.cronjobs[ns], *cj)
				}
			case "DELETED":
				if cjs, ok := rc.cronjobs[ns]; ok {
					for i, existing := range cjs {
						if existing.Name == cj.Name {
							rc.cronjobs[ns] = append(cjs[:i], cjs[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if cjs, ok := rc.cronjobs[ns]; ok {
					for i, existing := range cjs {
						if existing.Name == cj.Name {
							rc.cronjobs[ns][i] = *cj
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
	}
}

// watchIngresses watches for ingress changes across all namespaces
func (rc *ResourceCache) watchIngresses() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		watcher, err := rc.clientset.NetworkingV1().Ingresses("").Watch(rc.ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			ing, ok := event.Object.(*networkingv1.Ingress)
			if !ok {
				continue
			}

			rc.mu.Lock()
			ns := ing.Namespace
			switch event.Type {
			case "ADDED":
				if _, ok := rc.ingresses[ns]; !ok {
					rc.ingresses[ns] = []networkingv1.Ingress{}
				}
				exists := false
				for _, existing := range rc.ingresses[ns] {
					if existing.Name == ing.Name {
						exists = true
						break
					}
				}
				if !exists {
					rc.ingresses[ns] = append(rc.ingresses[ns], *ing)
				}
			case "DELETED":
				if ings, ok := rc.ingresses[ns]; ok {
					for i, existing := range ings {
						if existing.Name == ing.Name {
							rc.ingresses[ns] = append(ings[:i], ings[i+1:]...)
							break
						}
					}
				}
			case "MODIFIED":
				if ings, ok := rc.ingresses[ns]; ok {
					for i, existing := range ings {
						if existing.Name == ing.Name {
							rc.ingresses[ns][i] = *ing
							break
						}
					}
				}
			}
			rc.mu.Unlock()
		}

		time.Sleep(time.Second)
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
