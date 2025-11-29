package kubecomplete

// ClusterCache is your abstraction over client-go/cache.
type ClusterCache interface {
	Namespaces() []string
	// Example: resources you care about for "get" etc.
	ResourceTypes() []string
	// Optional: narrower list per verb (e.g. "rollout" → deployments, daemonsets, statefulsets)
	ResourceTypesForCommand(path []string) []string

	// Names for a given resource type in a namespace.
	ResourceNames(kind, namespace string) []string

	// Container names for a pod/workload target.
	Containers(namespace, resourceKind, resourceName string) []string
}
