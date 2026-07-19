package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type TopologySource string

const (
	TopologySourceFixture TopologySource = "fixture"
	TopologySourceNFD     TopologySource = "nfd"
	TopologySourceVendor  TopologySource = "vendor"
	TopologySourceDRA     TopologySource = "dra"
)

type AcceleratorVendor string

const (
	VendorAny    AcceleratorVendor = "any"
	VendorNVIDIA AcceleratorVendor = "nvidia"
	VendorHuawei AcceleratorVendor = "huawei"
	VendorAMD    AcceleratorVendor = "amd"
)

type Health string

const (
	HealthHealthy   Health = "Healthy"
	HealthUnhealthy Health = "Unhealthy"
	HealthUnknown   Health = "Unknown"
)

type LinkType string

const (
	LinkNVLink LinkType = "nvlink"
	LinkHCCS   LinkType = "hccs"
	LinkXGMI   LinkType = "xgmi"
	LinkPCIe   LinkType = "pcie"
)

type TopologyMode string

const (
	TopologyModeSingleNUMA      TopologyMode = "single-numa"
	TopologyModeSamePCIeRoot    TopologyMode = "same-pcie-root"
	TopologyModeFabricClique    TopologyMode = "fabric-clique"
	TopologyModeFabricConnected TopologyMode = "fabric-connected"
	TopologyModeBestEffort      TopologyMode = "best-effort"
)

type FailurePolicy string

const (
	FailurePolicyFailClosed FailurePolicy = "FailClosed"
	FailurePolicyBestEffort FailurePolicy = "BestEffort"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AcceleratorTopology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AcceleratorTopologySpec   `json:"spec"`
	Status AcceleratorTopologyStatus `json:"status,omitempty"`
}

type AcceleratorTopologySpec struct {
	NodeName           string              `json:"nodeName"`
	ObservedGeneration int64               `json:"observedGeneration"`
	Source             TopologySource      `json:"source"`
	Devices            []AcceleratorDevice `json:"devices"`
	Links              []AcceleratorLink   `json:"links,omitempty"`
}

type AcceleratorDevice struct {
	ID           string            `json:"id"`
	Vendor       AcceleratorVendor `json:"vendor"`
	Product      string            `json:"product"`
	ResourceName string            `json:"resourceName"`
	NUMANode     *int32            `json:"numaNode,omitempty"`
	PCIeRoot     string            `json:"pcieRoot,omitempty"`
	FabricGroup  string            `json:"fabricGroup,omitempty"`
	MemoryMiB    int64             `json:"memoryMiB"`
	Health       Health            `json:"health"`
}

type AcceleratorLink struct {
	Source        string   `json:"source"`
	Target        string   `json:"target"`
	Type          LinkType `json:"type"`
	BandwidthGBps float64  `json:"bandwidthGBps"`
	Hops          int32    `json:"hops"`
	Health        Health   `json:"health"`
}

type AcceleratorTopologyStatus struct {
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
	LastHeartbeatTime *metav1.Time       `json:"lastHeartbeatTime,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AcceleratorTopologyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AcceleratorTopology `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AcceleratorPlacementPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AcceleratorPlacementPolicySpec `json:"spec"`
}

type AcceleratorPlacementPolicySpec struct {
	Vendor                   AcceleratorVendor `json:"vendor,omitempty"`
	Products                 []string          `json:"products,omitempty"`
	MinimumMemoryMiB         int64             `json:"minimumMemoryMiB,omitempty"`
	TopologyMode             TopologyMode      `json:"topologyMode"`
	MinimumLinkBandwidthGBps float64           `json:"minimumLinkBandwidthGBps,omitempty"`
	MaxFabricHops            int32             `json:"maxFabricHops,omitempty"`
	AllowUnknownTopology     bool              `json:"allowUnknownTopology,omitempty"`
	Weights                  ScoreWeights      `json:"weights"`
	FailurePolicy            FailurePolicy     `json:"failurePolicy"`
}

type ScoreWeights struct {
	Locality      int32 `json:"locality"`
	Fabric        int32 `json:"fabric"`
	Fragmentation int32 `json:"fragmentation"`
	Headroom      int32 `json:"headroom"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AcceleratorPlacementPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AcceleratorPlacementPolicy `json:"items"`
}
