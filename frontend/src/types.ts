export type ViewId = "overview" | "fabric" | "decisions" | "dra" | "simulator";
export type ReplayMode = "baseline" | "fault";
export type Vendor = "nvidia" | "huawei" | "amd";
export type TopologyMode =
  | "single-numa"
  | "same-pcie-root"
  | "fabric-clique"
  | "fabric-connected"
  | "best-effort";

export interface Device {
  id: string;
  numa: number;
  pcieRoot: string;
  fabricGroup: string;
  memoryGiB: number;
  health: "Healthy" | "Unhealthy";
  reservedBy: string | null;
}

export interface FabricNode {
  id: string;
  shortName: string;
  vendor: Vendor;
  product: string;
  resourceName: string;
  linkType: "NVLink" | "HCCS" | "XGMI";
  bandwidthGBps: number;
  ready: boolean;
  heartbeat: string;
  devices: Device[];
}

export interface Candidate {
  nodeId: string;
  feasible: boolean;
  score: number | null;
  locality: number | null;
  fabric: number | null;
  fragmentation: number | null;
  headroom: number | null;
  selectedDevices: string[];
  reason: string | null;
}

export interface SchedulingStage {
  name: string;
  status: "complete" | "current" | "pending" | "failed";
  detail: string;
}

export interface Workload {
  id: string;
  namespace: string;
  vendor: Vendor | "any";
  deviceCount: number;
  topologyMode: TopologyMode;
  minimumBandwidthGBps: number;
  delivery: "DRA" | "Extended resource";
  state: "RUNNING" | "BOUND" | "WAITING" | "UNSCHEDULABLE";
  createdAt: string;
  selectedNodeId: string | null;
  policyName: string;
  podGroup: string | null;
  candidates: Candidate[];
  stages: SchedulingStage[];
}

export interface ResourceClaim {
  id: string;
  workloadId: string;
  nodeId: string;
  pool: string;
  devices: string[];
  state: "Prepared" | "Allocated" | "Pending";
  cdiVerified: boolean;
}

export interface GangMember {
  id: string;
  vendor: Vendor;
  nodeId: string | null;
  claimId: string;
  state: "Running" | "Waiting" | "Pending";
}

export interface PodGroup {
  id: string;
  minimumMembers: number;
  timeoutSeconds: number;
  phase: "Committed" | "Waiting" | "Rolled back";
  members: GangMember[];
}

export interface ActivityPoint {
  time: string;
  filter: number;
  reserve: number;
  bind: number;
}

export interface ReplaySnapshot {
  generatedAt: string;
  nodes: FabricNode[];
  workloads: Workload[];
  claims: ResourceClaim[];
  podGroups: PodGroup[];
  activity: ActivityPoint[];
}

export interface SimulationRequest {
  id: string;
  namespace: string;
  vendor: Vendor | "any";
  deviceCount: number;
  topologyMode: TopologyMode;
  minimumBandwidthGBps: number;
  delivery: "DRA" | "Extended resource";
  gangMembers: number;
}
