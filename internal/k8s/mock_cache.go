package k8s

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockResourceCache is a mock implementation of ResourceCache for testing/demo
type MockResourceCache struct {
	*ResourceCache
}

// NewMockResourceCache creates a new mock cache with fake data
func NewMockResourceCache() *MockResourceCache {
	rc := &ResourceCache{
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
		lastRefresh:  time.Now(),
	}

	// Populate with mock data
	rc.populateMockData()

	return &MockResourceCache{ResourceCache: rc}
}

// Start initializes the mock cache (no-op for mock)
func (mrc *MockResourceCache) Start(ctx context.Context) error {
	mrc.ctx, mrc.cancel = context.WithCancel(ctx)
	return nil
}

// populateMockData fills the cache with fake Kubernetes resources
func (rc *ResourceCache) populateMockData() {
	now := metav1.Now()
	oneHourAgo := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	oneDayAgo := metav1.NewTime(time.Now().Add(-24 * time.Hour))

	// Mock namespaces
	rc.namespaces = []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "default", CreationTimestamp: oneDayAgo}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system", CreationTimestamp: oneDayAgo}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-public", CreationTimestamp: oneDayAgo}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		{ObjectMeta: metav1.ObjectMeta{Name: "production", CreationTimestamp: oneDayAgo}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		{ObjectMeta: metav1.ObjectMeta{Name: "staging", CreationTimestamp: oneDayAgo}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		{ObjectMeta: metav1.ObjectMeta{Name: "development", CreationTimestamp: oneDayAgo}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
	}

	// Mock pods in default namespace
	rc.pods["default"] = []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "nginx-app-7d8f9c-abc12", Namespace: "default", CreationTimestamp: oneHourAgo}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "nginx-app-7d8f9c-def34", Namespace: "default", CreationTimestamp: oneHourAgo}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "backend-api-6b5c4d-xyz56", Namespace: "default", CreationTimestamp: oneHourAgo}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "frontend-web-8a7f2e-qrs78", Namespace: "default", CreationTimestamp: oneHourAgo}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "redis-cache-5c9d3a-mno90", Namespace: "default", CreationTimestamp: oneHourAgo}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	}

	// Mock pods in production namespace
	rc.pods["production"] = []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "my-app-prod-1a2b3c-xyz", Namespace: "production", CreationTimestamp: now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "my-app-prod-1a2b3c-abc", Namespace: "production", CreationTimestamp: now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		{ObjectMeta: metav1.ObjectMeta{Name: "database-primary-4d5e6f", Namespace: "production", CreationTimestamp: oneDayAgo}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	}

	replicas := int32(2)

	// Mock deployments
	rc.deployments["default"] = []appsv1.Deployment{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "nginx-app", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "backend-api", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "frontend-web", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
	}

	rc.deployments["production"] = []appsv1.Deployment{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "my-app-prod", Namespace: "production", CreationTimestamp: now},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
	}

	// Mock services
	rc.services["default"] = []corev1.Service{
		{ObjectMeta: metav1.ObjectMeta{Name: "nginx-service", Namespace: "default", CreationTimestamp: oneHourAgo}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}},
		{ObjectMeta: metav1.ObjectMeta{Name: "backend-api-service", Namespace: "default", CreationTimestamp: oneHourAgo}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}},
		{ObjectMeta: metav1.ObjectMeta{Name: "frontend-web-service", Namespace: "default", CreationTimestamp: oneHourAgo}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}},
	}

	// Mock StatefulSets
	rc.statefulsets["default"] = []appsv1.StatefulSet{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "redis-cluster", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
			Status:     appsv1.StatefulSetStatus{ReadyReplicas: 2},
		},
	}

	// Mock DaemonSets
	rc.daemonsets["kube-system"] = []appsv1.DaemonSet{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "kube-proxy", Namespace: "kube-system", CreationTimestamp: oneDayAgo},
			Status:     appsv1.DaemonSetStatus{NumberReady: 3, DesiredNumberScheduled: 3},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "fluentd", Namespace: "kube-system", CreationTimestamp: oneDayAgo},
			Status:     appsv1.DaemonSetStatus{NumberReady: 3, DesiredNumberScheduled: 3},
		},
	}

	// Mock ConfigMaps
	rc.configmaps["default"] = []corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "default", CreationTimestamp: oneHourAgo},
			Data:       map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "nginx-config", Namespace: "default", CreationTimestamp: oneHourAgo},
			Data:       map[string]string{"nginx.conf": "server {}"},
		},
	}

	// Mock Secrets
	rc.secrets["default"] = []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "db-credentials", Namespace: "default", CreationTimestamp: oneHourAgo},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"username": []byte("admin"), "password": []byte("secret")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "api-keys", Namespace: "default", CreationTimestamp: oneHourAgo},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"api-key": []byte("abc123")},
		},
	}

	// Mock Jobs
	completions := int32(1)
	rc.jobs["default"] = []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "data-migration-job", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec:       batchv1.JobSpec{Completions: &completions},
			Status:     batchv1.JobStatus{Succeeded: 1},
		},
	}

	// Mock CronJobs
	rc.cronjobs["default"] = []batchv1.CronJob{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "backup-cronjob", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec:       batchv1.CronJobSpec{Schedule: "0 2 * * *"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cleanup-cronjob", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec:       batchv1.CronJobSpec{Schedule: "0 */6 * * *"},
		},
	}

	// Mock Ingresses
	rc.ingresses["default"] = []networkingv1.Ingress{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "main-ingress", Namespace: "default", CreationTimestamp: oneHourAgo},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{Host: "example.com"},
					{Host: "api.example.com"},
				},
			},
		},
	}

	// Mock Nodes
	rc.nodes = []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-1", CreationTimestamp: oneDayAgo},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-2", CreationTimestamp: oneDayAgo},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-3", CreationTimestamp: oneDayAgo},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},
	}
}
