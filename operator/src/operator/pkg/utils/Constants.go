package utils

const (
	// OperationRemove instructs label remove operation.
	OperationRemove = "remove"
	// OperationAdd instructs label add operation.
	OperationAdd = "add"
	// RegistryService name of the registry service, expectation is daemonset of the service has the same name.
	RegistryService = "registry"
	// MetadataService name of the registry service, expectation is daemonset of the service has the same name.
	MetadataService = "metadata"
	// DataService name of the registry service, expectation is daemonset of the service has the same name.
	DataService = "data"
	// ClientService name of the registry service, expectation is daemonset of the service has the same name.
	ClientService = "client"

	// RegistryLabelKey selector of the registry service, expectation is daemonset of the service uses same label to start pod on node.
	RegistryLabelKey = "quobyte_registry"
	// MetadataLabelKey selector of the registry service, expectation is daemonset of the service uses same label to start pod on node.
	MetadataLabelKey = "quobyte_metadata"
	// DataLabelKey selector of the registry service, expectation is daemonset of the service uses same label to start pod on node.
	DataLabelKey = "quobyte_data"
	// ClientLabel selector of the registry service, expectation is daemonset of the service uses same label to start pod on node.
	ClientLabelKey = "quobyte_client"
	RegistrySelector = "role=registry"
	DataSelector = "role=data"
	MetadataSelector = "role=metadata"
	CLIENT_STATUS_FILE   = "/public/client-status.json"
	REGISTRY_STATUS_FILE = "/public/registry-status.json"
	DATA_STATUS_FILE     = "/public/data-status.json"
	METADATA_STATUS_FILE = "/public/metadata-status.json"
)
