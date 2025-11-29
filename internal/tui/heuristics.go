// File: internal/tui/heuristics.go

package tui

type CommandHeuristic struct {
	Command      string
	Description  string
	Verbs        []string // Alternative verbs
	RequiredArgs []ArgRequirement
	Flags        []FlagSpec
	Examples     []string
}

type ArgRequirement struct {
	Name             string
	Type             ArgType
	Required         bool
	Position         int
	CompletionSource CompletionSource
	Description      string
}

type ArgType int

const (
	ArgTypeResourceType ArgType = iota
	ArgTypeResourceName
	ArgTypeFile
	ArgTypeString
	ArgTypeInt
)

type CompletionSource int

const (
	CompletionNone CompletionSource = iota
	CompletionNamespace
	CompletionPod
	CompletionDeployment
	CompletionService
	CompletionNode
	CompletionConfigMap
	CompletionSecret
	CompletionFile
	CompletionContext
	CompletionContainer
	CompletionResourceType
)

type FlagSpec struct {
	Name          string
	Shorthand     string
	Type          FlagType
	Default       string
	Description   string
	Completion    CompletionSource
	RequiredWith  []string // Other flags that must be present
	ConflictsWith []string // Flags that cannot be used together
	AppliesTo     []string // Resource types this flag applies to
	Required      bool
}

type FlagType int

const (
	FlagTypeString FlagType = iota
	FlagTypeBool
	FlagTypeInt
	FlagTypeStringSlice
)

// The complete heuristics map
var KubectlHeuristics = map[string]CommandHeuristic{

	// GETTING STARTED COMMANDS
	"create": {
		Command:     "create",
		Description: "Create a resource from a file or stdin",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: false, Position: 0, CompletionSource: CompletionResourceType},
		},
		Flags: []FlagSpec{
			{Name: "filename", Shorthand: "f", Type: FlagTypeStringSlice, Completion: CompletionFile, Description: "Filename, directory, or URL to files"},
			{Name: "kustomize", Shorthand: "k", Type: FlagTypeString, Completion: CompletionFile, Description: "Process kustomization directory"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none", Description: "Must be 'none', 'server', or 'client'"},
			{Name: "output", Shorthand: "o", Type: FlagTypeString, Description: "Output format"},
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace, Description: "Namespace to use"},
			{Name: "edit", Shorthand: "", Type: FlagTypeBool, Description: "Edit the API resource before creating"},
			{Name: "save-config", Shorthand: "", Type: FlagTypeBool, Description: "Save config in annotation"},
		},
	},

	"get": {
		Command:     "get",
		Description: "Display one or many resources",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: false, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: false, Position: 1, CompletionSource: CompletionNone}, // Dynamically determined based on resource type
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace, Description: "Namespace"},
			{Name: "all-namespaces", Shorthand: "A", Type: FlagTypeBool, Description: "List across all namespaces", ConflictsWith: []string{"namespace"}},
			{Name: "output", Shorthand: "o", Type: FlagTypeString, Description: "Output format (json|yaml|wide|name|custom-columns|etc)"},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString, Description: "Label selector"},
			{Name: "field-selector", Shorthand: "", Type: FlagTypeString, Description: "Field selector"},
			{Name: "watch", Shorthand: "w", Type: FlagTypeBool, Description: "Watch for changes"},
			{Name: "watch-only", Shorthand: "", Type: FlagTypeBool, Description: "Watch for changes without listing first"},
			{Name: "show-labels", Shorthand: "", Type: FlagTypeBool, Description: "Show all labels"},
			{Name: "sort-by", Shorthand: "", Type: FlagTypeString, Description: "Sort by JSONPath expression"},
			{Name: "no-headers", Shorthand: "", Type: FlagTypeBool, Description: "Don't print headers"},
			{Name: "chunk-size", Shorthand: "", Type: FlagTypeInt, Default: "500", Description: "Chunk size for large lists"},
		},
	},

	"describe": {
		Command:     "describe",
		Description: "Show details of a specific resource or group of resources",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: false, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: false, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "all-namespaces", Shorthand: "A", Type: FlagTypeBool, ConflictsWith: []string{"namespace"}},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString, Description: "Label selector"},
			{Name: "show-events", Shorthand: "", Type: FlagTypeBool, Default: "true", Description: "Show events"},
		},
	},

	"delete": {
		Command:     "delete",
		Description: "Delete resources by filenames, stdin, resources and names, or by resources and label selector",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: false, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: false, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "filename", Shorthand: "f", Type: FlagTypeStringSlice, Completion: CompletionFile},
			{Name: "kustomize", Shorthand: "k", Type: FlagTypeString, Completion: CompletionFile},
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "all-namespaces", Shorthand: "A", Type: FlagTypeBool, ConflictsWith: []string{"namespace"}},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString, Description: "Label selector"},
			{Name: "all", Shorthand: "", Type: FlagTypeBool, Description: "Delete all resources"},
			{Name: "force", Shorthand: "", Type: FlagTypeBool, Description: "Force deletion"},
			{Name: "grace-period", Shorthand: "", Type: FlagTypeInt, Default: "-1", Description: "Grace period in seconds"},
			{Name: "now", Shorthand: "", Type: FlagTypeBool, Description: "Immediate shutdown"},
			{Name: "wait", Shorthand: "", Type: FlagTypeBool, Default: "true", Description: "Wait for deletion"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"apply": {
		Command:     "apply",
		Description: "Apply a configuration to a resource by filename or stdin",
		Flags: []FlagSpec{
			{Name: "filename", Shorthand: "f", Type: FlagTypeStringSlice, Completion: CompletionFile, Description: "Files to apply"},
			{Name: "kustomize", Shorthand: "k", Type: FlagTypeString, Completion: CompletionFile},
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "recursive", Shorthand: "R", Type: FlagTypeBool, Description: "Process directories recursively"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
			{Name: "force", Shorthand: "", Type: FlagTypeBool, Description: "Force apply"},
			{Name: "server-side", Shorthand: "", Type: FlagTypeBool, Description: "Server-side apply"},
			{Name: "force-conflicts", Shorthand: "", Type: FlagTypeBool, Description: "Force conflicts in server-side apply"},
			{Name: "prune", Shorthand: "", Type: FlagTypeBool, Description: "Prune resources not in file"},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString, RequiredWith: []string{"prune"}},
			{Name: "wait", Shorthand: "", Type: FlagTypeBool, Description: "Wait for resources"},
		},
	},

	// WORKING WITH APPS
	"logs": {
		Command:     "logs",
		Description: "Print the logs for a container in a pod",
		RequiredArgs: []ArgRequirement{
			{Name: "podName", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionPod},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "container", Shorthand: "c", Type: FlagTypeString, Completion: CompletionContainer, Description: "Container name"},
			{Name: "follow", Shorthand: "f", Type: FlagTypeBool, Description: "Stream logs"},
			{Name: "previous", Shorthand: "p", Type: FlagTypeBool, Description: "Previous instance"},
			{Name: "since", Shorthand: "", Type: FlagTypeString, Description: "Time duration (e.g., 5s, 2m, 3h)"},
			{Name: "since-time", Shorthand: "", Type: FlagTypeString, Description: "RFC3339 timestamp"},
			{Name: "timestamps", Shorthand: "", Type: FlagTypeBool, Description: "Include timestamps"},
			{Name: "tail", Shorthand: "", Type: FlagTypeInt, Default: "-1", Description: "Lines to show from end"},
			{Name: "limit-bytes", Shorthand: "", Type: FlagTypeInt, Description: "Max bytes to return"},
			{Name: "all-containers", Shorthand: "", Type: FlagTypeBool, Description: "All containers in pod"},
			{Name: "prefix", Shorthand: "", Type: FlagTypeBool, Description: "Prefix lines with container name"},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString, Description: "Label selector"},
		},
	},

	"exec": {
		Command:     "exec",
		Description: "Execute a command in a container",
		RequiredArgs: []ArgRequirement{
			{Name: "podName", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionPod},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "container", Shorthand: "c", Type: FlagTypeString, Completion: CompletionContainer},
			{Name: "stdin", Shorthand: "i", Type: FlagTypeBool, Description: "Pass stdin"},
			{Name: "tty", Shorthand: "t", Type: FlagTypeBool, Description: "Allocate TTY"},
		},
	},

	"port-forward": {
		Command:     "port-forward",
		Description: "Forward one or more local ports to a pod",
		RequiredArgs: []ArgRequirement{
			{Name: "resource", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionPod, Description: "Pod or service name"}, // Can also be service
			{Name: "portSpec", Type: ArgTypeString, Required: true, Position: 1, Description: "local:remote port mapping"},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "address", Shorthand: "", Type: FlagTypeStringSlice, Default: "[localhost]", Description: "Addresses to listen on"},
		},
	},

	"attach": {
		Command:     "attach",
		Description: "Attach to a running container",
		RequiredArgs: []ArgRequirement{
			{Name: "podName", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionPod},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "container", Shorthand: "c", Type: FlagTypeString, Completion: CompletionContainer},
			{Name: "stdin", Shorthand: "i", Type: FlagTypeBool},
			{Name: "tty", Shorthand: "t", Type: FlagTypeBool},
		},
	},

	"cp": {
		Command:     "cp",
		Description: "Copy files and directories to and from containers",
		RequiredArgs: []ArgRequirement{
			{Name: "source", Type: ArgTypeString, Required: true, Position: 0},
			{Name: "dest", Type: ArgTypeString, Required: true, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "container", Shorthand: "c", Type: FlagTypeString, Completion: CompletionContainer},
			{Name: "no-preserve", Shorthand: "", Type: FlagTypeBool, Description: "Don't preserve permissions"},
		},
	},

	// CLUSTER MANAGEMENT
	"top": {
		Command:     "top",
		Description: "Display resource usage",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType, Description: "node or pod"},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace, AppliesTo: []string{"pod"}},
			{Name: "all-namespaces", Shorthand: "A", Type: FlagTypeBool, AppliesTo: []string{"pod"}},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString, AppliesTo: []string{"pod"}},
			{Name: "containers", Shorthand: "", Type: FlagTypeBool, AppliesTo: []string{"pod"}, Description: "Show container metrics"},
			{Name: "sort-by", Shorthand: "", Type: FlagTypeString, Description: "Sort by cpu or memory"},
			{Name: "no-headers", Shorthand: "", Type: FlagTypeBool},
		},
	},

	"cordon": {
		Command:     "cordon",
		Description: "Mark node as unschedulable",
		RequiredArgs: []ArgRequirement{
			{Name: "nodeName", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionNode},
		},
		Flags: []FlagSpec{
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
		},
	},

	"uncordon": {
		Command:     "uncordon",
		Description: "Mark node as schedulable",
		RequiredArgs: []ArgRequirement{
			{Name: "nodeName", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionNode},
		},
		Flags: []FlagSpec{
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
		},
	},

	"drain": {
		Command:     "drain",
		Description: "Drain node in preparation for maintenance",
		RequiredArgs: []ArgRequirement{
			{Name: "nodeName", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionNode},
		},
		Flags: []FlagSpec{
			{Name: "force", Shorthand: "", Type: FlagTypeBool, Description: "Force drain"},
			{Name: "ignore-daemonsets", Shorthand: "", Type: FlagTypeBool, Description: "Ignore DaemonSets"},
			{Name: "delete-emptydir-data", Shorthand: "", Type: FlagTypeBool, Description: "Delete emptyDir data"},
			{Name: "disable-eviction", Shorthand: "", Type: FlagTypeBool, Description: "Use delete instead of evict"},
			{Name: "grace-period", Shorthand: "", Type: FlagTypeInt, Default: "-1"},
			{Name: "timeout", Shorthand: "", Type: FlagTypeString, Default: "0s"},
			{Name: "pod-selector", Shorthand: "", Type: FlagTypeString, Description: "Label selector for pods"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"taint": {
		Command:     "taint",
		Description: "Update the taints on one or more nodes",
		RequiredArgs: []ArgRequirement{
			{Name: "nodeName", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionNode},
			{Name: "taintSpec", Type: ArgTypeString, Required: true, Position: 1, Description: "key=value:effect"},
		},
		Flags: []FlagSpec{
			{Name: "all", Shorthand: "", Type: FlagTypeBool, Description: "Taint all nodes"},
			{Name: "overwrite", Shorthand: "", Type: FlagTypeBool, Description: "Overwrite existing taint"},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"label": {
		Command:     "label",
		Description: "Update labels on a resource",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: true, Position: 1},
			{Name: "labels", Type: ArgTypeString, Required: true, Position: 2, Description: "key=value or key-"},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "all", Shorthand: "", Type: FlagTypeBool, Description: "All resources"},
			{Name: "overwrite", Shorthand: "", Type: FlagTypeBool, Description: "Overwrite existing labels"},
			{Name: "resource-version", Shorthand: "", Type: FlagTypeString, Description: "Resource version"},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"annotate": {
		Command:     "annotate",
		Description: "Update annotations on a resource",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: true, Position: 1},
			{Name: "annotations", Type: ArgTypeString, Required: true, Position: 2, Description: "key=value or key-"},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "all", Shorthand: "", Type: FlagTypeBool},
			{Name: "overwrite", Shorthand: "", Type: FlagTypeBool},
			{Name: "resource-version", Shorthand: "", Type: FlagTypeString},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	// DEPLOY COMMANDS
	"rollout": {
		Command:     "rollout",
		Description: "Manage rollout of a resource",
		RequiredArgs: []ArgRequirement{
			{Name: "subcommand", Type: ArgTypeString, Required: true, Position: 0, Description: "status|history|pause|resume|restart|undo"},
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 1, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: false, Position: 2},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "revision", Shorthand: "", Type: FlagTypeInt, Description: "Revision number", AppliesTo: []string{"history", "undo"}},
			{Name: "to-revision", Shorthand: "", Type: FlagTypeInt, Description: "Rollback to revision", AppliesTo: []string{"undo"}},
			{Name: "watch", Shorthand: "w", Type: FlagTypeBool, AppliesTo: []string{"status"}},
			{Name: "timeout", Shorthand: "", Type: FlagTypeString, AppliesTo: []string{"status"}},
		},
	},

	"scale": {
		Command:     "scale",
		Description: "Set new size for a resource",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: true, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "replicas", Shorthand: "", Type: FlagTypeInt, Required: true, Description: "New replica count"},
			{Name: "current-replicas", Shorthand: "", Type: FlagTypeInt, Description: "Precondition for current size"},
			{Name: "resource-version", Shorthand: "", Type: FlagTypeString, Description: "Precondition for resource version"},
			{Name: "timeout", Shorthand: "", Type: FlagTypeString, Default: "0s"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"autoscale": {
		Command:     "autoscale",
		Description: "Auto-scale a resource",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: true, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "min", Shorthand: "", Type: FlagTypeInt, Description: "Minimum pods"},
			{Name: "max", Shorthand: "", Type: FlagTypeInt, Required: true, Description: "Maximum pods"},
			{Name: "cpu-percent", Shorthand: "", Type: FlagTypeInt, Description: "Target CPU utilization"},
			{Name: "name", Shorthand: "", Type: FlagTypeString, Description: "Name for HPA"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"expose": {
		Command:     "expose",
		Description: "Expose a resource as a new service",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: true, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "port", Shorthand: "", Type: FlagTypeInt, Description: "Service port"},
			{Name: "target-port", Shorthand: "", Type: FlagTypeString, Description: "Target port"},
			{Name: "protocol", Shorthand: "", Type: FlagTypeString, Default: "TCP", Description: "TCP|UDP|SCTP"},
			{Name: "type", Shorthand: "", Type: FlagTypeString, Description: "ClusterIP|NodePort|LoadBalancer|ExternalName"},
			{Name: "name", Shorthand: "", Type: FlagTypeString, Description: "Service name"},
			{Name: "selector", Shorthand: "", Type: FlagTypeString, Description: "Label selector"},
			{Name: "external-ip", Shorthand: "", Type: FlagTypeString, Description: "External IP"},
			{Name: "load-balancer-ip", Shorthand: "", Type: FlagTypeString, Description: "LoadBalancer IP"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"set": {
		Command:     "set",
		Description: "Set specific features on objects",
		RequiredArgs: []ArgRequirement{
			{Name: "subcommand", Type: ArgTypeString, Required: true, Position: 0, Description: "image|resources|selector|serviceaccount|subject|env"},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "all", Shorthand: "", Type: FlagTypeBool},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
			{Name: "local", Shorthand: "", Type: FlagTypeBool},
		},
	},

	"run": {
		Command:     "run",
		Description: "Run a particular image on the cluster",
		RequiredArgs: []ArgRequirement{
			{Name: "name", Type: ArgTypeString, Required: true, Position: 0, Description: "Pod name"},
		},
		Flags: []FlagSpec{
			{Name: "image", Shorthand: "", Type: FlagTypeString, Required: true, Description: "Image to run"},
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "port", Shorthand: "", Type: FlagTypeInt, Description: "Port to expose"},
			{Name: "env", Shorthand: "", Type: FlagTypeStringSlice, Description: "Environment variables"},
			{Name: "labels", Shorthand: "l", Type: FlagTypeString, Description: "Labels"},
			{Name: "annotations", Shorthand: "", Type: FlagTypeStringSlice, Description: "Annotations"},
			{Name: "restart", Shorthand: "", Type: FlagTypeString, Default: "Always", Description: "Always|OnFailure|Never"},
			{Name: "command", Shorthand: "", Type: FlagTypeBool, Description: "Use command instead of args"},
			{Name: "stdin", Shorthand: "i", Type: FlagTypeBool},
			{Name: "tty", Shorthand: "t", Type: FlagTypeBool},
			{Name: "rm", Shorthand: "", Type: FlagTypeBool, Description: "Delete after exit"},
			{Name: "attach", Shorthand: "", Type: FlagTypeBool, Description: "Attach to pod"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	// ADVANCED COMMANDS
	"diff": {
		Command:     "diff",
		Description: "Diff live version against would-be applied version",
		Flags: []FlagSpec{
			{Name: "filename", Shorthand: "f", Type: FlagTypeStringSlice, Completion: CompletionFile, Required: true},
			{Name: "kustomize", Shorthand: "k", Type: FlagTypeString, Completion: CompletionFile},
			{Name: "recursive", Shorthand: "R", Type: FlagTypeBool},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
			{Name: "server-side", Shorthand: "", Type: FlagTypeBool},
		},
	},

	"edit": {
		Command:     "edit",
		Description: "Edit a resource on the server",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: true, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "output", Shorthand: "o", Type: FlagTypeString, Default: "yaml", Description: "Output format"},
			{Name: "output-patch", Shorthand: "", Type: FlagTypeBool, Description: "Show the patch"},
			{Name: "save-config", Shorthand: "", Type: FlagTypeBool},
		},
	},

	"patch": {
		Command:     "patch",
		Description: "Update field(s) of a resource using strategic merge patch",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: true, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "patch", Shorthand: "p", Type: FlagTypeString, Description: "Patch string"},
			{Name: "patch-file", Shorthand: "", Type: FlagTypeString, Completion: CompletionFile, Description: "Patch file"},
			{Name: "type", Shorthand: "", Type: FlagTypeString, Default: "strategic", Description: "strategic|merge|json"},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"replace": {
		Command:     "replace",
		Description: "Replace a resource by filename or stdin",
		Flags: []FlagSpec{
			{Name: "filename", Shorthand: "f", Type: FlagTypeStringSlice, Completion: CompletionFile, Required: true},
			{Name: "kustomize", Shorthand: "k", Type: FlagTypeString, Completion: CompletionFile},
			{Name: "force", Shorthand: "", Type: FlagTypeBool, Description: "Force replace (delete and recreate)"},
			{Name: "cascade", Shorthand: "", Type: FlagTypeString, Default: "background", Description: "background|orphan|foreground"},
			{Name: "grace-period", Shorthand: "", Type: FlagTypeInt, Default: "-1"},
			{Name: "save-config", Shorthand: "", Type: FlagTypeBool},
			{Name: "dry-run", Shorthand: "", Type: FlagTypeString, Default: "none"},
		},
	},

	"wait": {
		Command:     "wait",
		Description: "Wait for a specific condition on one or many resources",
		RequiredArgs: []ArgRequirement{
			{Name: "resourceType", Type: ArgTypeResourceType, Required: true, Position: 0, CompletionSource: CompletionResourceType},
			{Name: "resourceName", Type: ArgTypeResourceName, Required: false, Position: 1},
		},
		Flags: []FlagSpec{
			{Name: "for", Shorthand: "", Type: FlagTypeString, Required: true, Description: "Condition to wait for"},
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "selector", Shorthand: "l", Type: FlagTypeString},
			{Name: "all", Shorthand: "", Type: FlagTypeBool},
			{Name: "timeout", Shorthand: "", Type: FlagTypeString, Default: "30s"},
		},
	},

	"debug": {
		Command:     "debug",
		Description: "Create debugging sessions for troubleshooting workloads and nodes",
		RequiredArgs: []ArgRequirement{
			{Name: "resource", Type: ArgTypeResourceName, Required: true, Position: 0, CompletionSource: CompletionPod},
		},
		Flags: []FlagSpec{
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Completion: CompletionNamespace},
			{Name: "container", Shorthand: "c", Type: FlagTypeString, Completion: CompletionContainer},
			{Name: "image", Shorthand: "", Type: FlagTypeString, Description: "Debug image"},
			{Name: "stdin", Shorthand: "i", Type: FlagTypeBool},
			{Name: "tty", Shorthand: "t", Type: FlagTypeBool},
			{Name: "attach", Shorthand: "", Type: FlagTypeBool},
			{Name: "copy-to", Shorthand: "", Type: FlagTypeString, Description: "Copy pod with this name"},
			{Name: "replace", Shorthand: "", Type: FlagTypeBool, Description: "Delete original pod"},
			{Name: "same-node", Shorthand: "", Type: FlagTypeBool, Description: "Schedule on same node"},
			{Name: "set-image", Shorthand: "", Type: FlagTypeStringSlice, Description: "Container images to set"},
			{Name: "share-processes", Shorthand: "", Type: FlagTypeBool, Default: "true"},
		},
	},

	// CONFIG COMMANDS
	"config": {
		Command:     "config",
		Description: "Modify kubeconfig files",
		RequiredArgs: []ArgRequirement{
			{Name: "subcommand", Type: ArgTypeString, Required: true, Position: 0, Description: "view|get-contexts|current-context|use-context|set-context|set-cluster|set-credentials|unset|rename-context|delete-context|delete-cluster|delete-user"},
		},
		Flags: []FlagSpec{
			{Name: "kubeconfig", Shorthand: "", Type: FlagTypeString, Completion: CompletionFile, Description: "Path to kubeconfig"},
			{Name: "context", Shorthand: "", Type: FlagTypeString, Completion: CompletionContext, Description: "Context name"},
			{Name: "cluster", Shorthand: "", Type: FlagTypeString, Description: "Cluster name"},
			{Name: "user", Shorthand: "", Type: FlagTypeString, Description: "User name"},
			{Name: "namespace", Shorthand: "n", Type: FlagTypeString, Description: "Namespace"},
			{Name: "current", Shorthand: "", Type: FlagTypeBool, Description: "Modify current context"},
			{Name: "output", Shorthand: "o", Type: FlagTypeString, AppliesTo: []string{"view"}},
			{Name: "minify", Shorthand: "", Type: FlagTypeBool, AppliesTo: []string{"view"}},
			{Name: "raw", Shorthand: "", Type: FlagTypeBool, AppliesTo: []string{"view"}},
			{Name: "flatten", Shorthand: "", Type: FlagTypeBool, AppliesTo: []string{"view"}},
		},
	},

	"cluster-info": {
		Command:     "cluster-info",
		Description: "Display cluster information",
		RequiredArgs: []ArgRequirement{
			{Name: "subcommand", Type: ArgTypeString, Required: false, Position: 0, Description: "dump"},
		},
		Flags: []FlagSpec{
			{Name: "output-directory", Shorthand: "", Type: FlagTypeString, Completion: CompletionFile, AppliesTo: []string{"dump"}},
		},
	},

	"version": {
		Command:     "version",
		Description: "Print the client and server version information",
		Flags: []FlagSpec{
			{Name: "client", Shorthand: "", Type: FlagTypeBool, Description: "Client version only"},
			{Name: "output", Shorthand: "o", Type: FlagTypeString, Description: "json|yaml"},
			{Name: "short", Shorthand: "", Type: FlagTypeBool, Description: "Short version"},
		},
	},

	"api-resources": {
		Command:     "api-resources",
		Description: "Print the supported API resources",
		Flags: []FlagSpec{
			{Name: "namespaced", Shorthand: "", Type: FlagTypeBool, Description: "Filter by namespaced"},
			{Name: "api-group", Shorthand: "", Type: FlagTypeString, Description: "Filter by API group"},
			{Name: "verbs", Shorthand: "", Type: FlagTypeStringSlice, Description: "Filter by supported verbs"},
			{Name: "output", Shorthand: "o", Type: FlagTypeString, Description: "wide|name"},
			{Name: "cached", Shorthand: "", Type: FlagTypeBool, Description: "Use cached list"},
			{Name: "sort-by", Shorthand: "", Type: FlagTypeString, Description: "name|kind"},
		},
	},

	"api-versions": {
		Command:     "api-versions",
		Description: "Print the supported API versions",
		Flags:       []FlagSpec{},
	},

	"explain": {
		Command:     "explain",
		Description: "Get documentation for a resource",
		RequiredArgs: []ArgRequirement{
			{Name: "resource", Type: ArgTypeString, Required: true, Position: 0, Description: "Resource type or field path"},
		},
		Flags: []FlagSpec{
			{Name: "recursive", Shorthand: "", Type: FlagTypeBool, Description: "Show all fields recursively"},
			{Name: "output", Shorthand: "", Type: FlagTypeString, Description: "plaintext-openapiv2"},
		},
	},
}

// Resource type completions
var ResourceTypeCompletions = []string{
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

// Output format completions
var OutputFormatCompletions = []string{
	"json",
	"yaml",
	"wide",
	"name",
	"custom-columns",
	"custom-columns-file",
	"go-template",
	"go-template-file",
	"jsonpath",
	"jsonpath-file",
}

// Dry-run values
var DryRunValues = []string{
	"none",
	"client",
	"server",
}

// Helper function to get command heuristic
func GetCommandHeuristic(cmd string) (CommandHeuristic, bool) {
	h, ok := KubectlHeuristics[cmd]
	return h, ok
}

// Helper function to get applicable flags for a command
func GetApplicableFlags(cmd string, resourceType string) []FlagSpec {
	h, ok := KubectlHeuristics[cmd]
	if !ok {
		return nil
	}

	var applicable []FlagSpec
	for _, flag := range h.Flags {
		// If flag has no specific resource types, it applies to all
		if len(flag.AppliesTo) == 0 {
			applicable = append(applicable, flag)
			continue
		}

		// Check if current resource type is in AppliesTo list
		for _, rt := range flag.AppliesTo {
			if rt == resourceType {
				applicable = append(applicable, flag)
				break
			}
		}
	}

	return applicable
}

// Helper function to get completion source for a flag
func GetFlagCompletion(cmd, flagName string) CompletionSource {
	h, ok := KubectlHeuristics[cmd]
	if !ok {
		return CompletionNone
	}

	for _, flag := range h.Flags {
		if flag.Name == flagName || flag.Shorthand == flagName {
			return flag.Completion
		}
	}

	return CompletionNone
}
