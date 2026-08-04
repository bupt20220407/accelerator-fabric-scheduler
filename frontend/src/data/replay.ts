import type {
  Candidate,
  FabricNode,
  ReplaySnapshot,
  SchedulingStage,
  SimulationRequest,
  Workload,
} from "../types";

const createdAt = "2026-07-20T10:42:00Z";

function makeDevices(prefix: string, memoryGiB: number) {
  return Array.from({ length: 8 }, (_, index) => ({
    id: `${prefix}${index}`,
    numa: Math.floor(index / 4),
    pcieRoot: index < 4 ? "0000:30" : "0000:80",
    fabricGroup: `${index < 4 ? "clique-0" : "clique-1"}`,
    memoryGiB,
    health: "Healthy" as const,
    reservedBy: null,
  }));
}

const baseNodes: FabricNode[] = [
  {
    id: "accelerator-fabric-worker",
    shortName: "worker-nvidia",
    vendor: "nvidia",
    product: "H100-SXM",
    resourceName: "nvidia.com/gpu",
    linkType: "NVLink",
    bandwidthGBps: 450,
    ready: true,
    heartbeat: "2026-07-20T10:41:54Z",
    devices: makeDevices("gpu", 80),
  },
  {
    id: "accelerator-fabric-worker2",
    shortName: "worker-ascend",
    vendor: "huawei",
    product: "Ascend-910B",
    resourceName: "huawei.com/Ascend910",
    linkType: "HCCS",
    bandwidthGBps: 392,
    ready: true,
    heartbeat: "2026-07-20T10:41:52Z",
    devices: makeDevices("ascend", 64),
  },
  {
    id: "accelerator-fabric-worker3",
    shortName: "worker-amd",
    vendor: "amd",
    product: "MI300X",
    resourceName: "amd.com/gpu",
    linkType: "XGMI",
    bandwidthGBps: 400,
    ready: true,
    heartbeat: "2026-07-20T10:41:50Z",
    devices: makeDevices("gpu", 192),
  },
];

function candidate(
  nodeId: string,
  feasible: boolean,
  score: number | null,
  selectedDevices: string[],
  reason: string | null = null,
): Candidate {
  return {
    nodeId,
    feasible,
    score,
    locality: score === null ? null : Math.min(100, score + 7),
    fabric: score === null ? null : Math.min(100, score + 4),
    fragmentation: score === null ? null : Math.max(0, score - 8),
    headroom: score === null ? null : Math.max(0, score - 14),
    selectedDevices,
    reason,
  };
}

const boundStages: SchedulingStage[] = [
  { name: "PreFilter", status: "complete", detail: "Policy and accelerator request resolved." },
  { name: "Filter", status: "complete", detail: "Topology constraints satisfied." },
  { name: "Score", status: "complete", detail: "Weighted node features normalized." },
  { name: "Reserve", status: "complete", detail: "Advisory device combination reserved." },
  { name: "Permit", status: "complete", detail: "PodGroup admission barrier cleared." },
  { name: "PreBind", status: "complete", detail: "Reservation ownership verified." },
];

const workloads: Workload[] = [
  {
    id: "llm-pretrain-rank-a",
    namespace: "training",
    vendor: "nvidia",
    deviceCount: 4,
    topologyMode: "fabric-clique",
    minimumBandwidthGBps: 300,
    delivery: "DRA",
    state: "RUNNING",
    createdAt: "2026-07-20T10:36:00Z",
    selectedNodeId: "accelerator-fabric-worker",
    policyName: "high-bandwidth-training",
    podGroup: "llm-pretrain",
    candidates: [
      candidate("accelerator-fabric-worker", true, 94, ["gpu0", "gpu1", "gpu2", "gpu3"]),
      candidate("accelerator-fabric-worker2", false, null, [], "Vendor huawei does not satisfy nvidia."),
      candidate("accelerator-fabric-worker3", false, null, [], "Vendor amd does not satisfy nvidia."),
    ],
    stages: boundStages,
  },
  {
    id: "recommendation-batch-17",
    namespace: "inference",
    vendor: "amd",
    deviceCount: 2,
    topologyMode: "same-pcie-root",
    minimumBandwidthGBps: 64,
    delivery: "DRA",
    state: "BOUND",
    createdAt: "2026-07-20T10:32:00Z",
    selectedNodeId: "accelerator-fabric-worker3",
    policyName: "amd-local-inference",
    podGroup: null,
    candidates: [
      candidate("accelerator-fabric-worker", false, null, [], "Vendor nvidia does not satisfy amd."),
      candidate("accelerator-fabric-worker2", false, null, [], "Vendor huawei does not satisfy amd."),
      candidate("accelerator-fabric-worker3", true, 88, ["gpu4", "gpu5"]),
    ],
    stages: boundStages,
  },
  {
    id: "heterogeneous-rank-huawei",
    namespace: "research",
    vendor: "huawei",
    deviceCount: 1,
    topologyMode: "best-effort",
    minimumBandwidthGBps: 0,
    delivery: "DRA",
    state: "RUNNING",
    createdAt: "2026-07-20T10:28:00Z",
    selectedNodeId: "accelerator-fabric-worker2",
    policyName: "heterogeneous-gang-huawei",
    podGroup: "heterogeneous-dra-3",
    candidates: [
      candidate("accelerator-fabric-worker", false, null, [], "Vendor nvidia does not satisfy huawei."),
      candidate("accelerator-fabric-worker2", true, 91, ["ascend0"]),
      candidate("accelerator-fabric-worker3", false, null, [], "Vendor amd does not satisfy huawei."),
    ],
    stages: boundStages,
  },
  {
    id: "five-card-clique-probe",
    namespace: "validation",
    vendor: "nvidia",
    deviceCount: 5,
    topologyMode: "fabric-clique",
    minimumBandwidthGBps: 300,
    delivery: "Extended resource",
    state: "UNSCHEDULABLE",
    createdAt: "2026-07-20T10:39:00Z",
    selectedNodeId: null,
    policyName: "high-bandwidth-training",
    podGroup: null,
    candidates: [
      candidate("accelerator-fabric-worker", false, null, [], "No five-device fabric clique exists; maximum clique size is four."),
      candidate("accelerator-fabric-worker2", false, null, [], "Vendor huawei does not satisfy nvidia."),
      candidate("accelerator-fabric-worker3", false, null, [], "Vendor amd does not satisfy nvidia."),
    ],
    stages: [
      { name: "PreFilter", status: "complete", detail: "Policy and five-device request resolved." },
      { name: "Filter", status: "failed", detail: "All nodes rejected the fabric-clique constraint." },
      { name: "Score", status: "pending", detail: "No feasible nodes to score." },
      { name: "Reserve", status: "pending", detail: "No reservation created." },
      { name: "Permit", status: "pending", detail: "Not reached." },
      { name: "PreBind", status: "pending", detail: "Not reached." },
    ],
  },
];

function reserve(nodes: FabricNode[], nodeId: string, devices: string[], owner: string) {
  return nodes.map((node) =>
    node.id !== nodeId
      ? node
      : {
          ...node,
          devices: node.devices.map((device) =>
            devices.includes(device.id) ? { ...device, reservedBy: owner } : device,
          ),
        },
  );
}

let nodes = reserve(baseNodes, "accelerator-fabric-worker", ["gpu0", "gpu1", "gpu2", "gpu3"], "llm-pretrain-rank-a");
nodes = reserve(nodes, "accelerator-fabric-worker3", ["gpu4", "gpu5"], "recommendation-batch-17");
nodes = reserve(nodes, "accelerator-fabric-worker2", ["ascend0"], "heterogeneous-rank-huawei");

export const replaySnapshot: ReplaySnapshot = {
  generatedAt: createdAt,
  nodes,
  workloads,
  claims: [
    {
      id: "claim-llm-pretrain-a",
      workloadId: "llm-pretrain-rank-a",
      nodeId: "accelerator-fabric-worker",
      pool: "accelerator-fabric-worker",
      devices: ["gpu0", "gpu1", "gpu2", "gpu3"],
      state: "Prepared",
      cdiVerified: true,
    },
    {
      id: "claim-recommendation-17",
      workloadId: "recommendation-batch-17",
      nodeId: "accelerator-fabric-worker3",
      pool: "accelerator-fabric-worker3",
      devices: ["gpu4", "gpu5"],
      state: "Prepared",
      cdiVerified: true,
    },
    {
      id: "claim-heterogeneous-huawei",
      workloadId: "heterogeneous-rank-huawei",
      nodeId: "accelerator-fabric-worker2",
      pool: "accelerator-fabric-worker2",
      devices: ["ascend0"],
      state: "Prepared",
      cdiVerified: true,
    },
  ],
  podGroups: [
    {
      id: "heterogeneous-dra-3",
      minimumMembers: 3,
      timeoutSeconds: 20,
      phase: "Committed",
      members: [
        { id: "rank-nvidia", vendor: "nvidia", nodeId: "accelerator-fabric-worker", claimId: "claim-heterogeneous-nvidia", state: "Running" },
        { id: "rank-huawei", vendor: "huawei", nodeId: "accelerator-fabric-worker2", claimId: "claim-heterogeneous-huawei", state: "Running" },
        { id: "rank-amd", vendor: "amd", nodeId: "accelerator-fabric-worker3", claimId: "claim-heterogeneous-amd", state: "Running" },
      ],
    },
  ],
  activity: [
    { time: "10:00", filter: 34, reserve: 12, bind: 10 },
    { time: "10:10", filter: 41, reserve: 16, bind: 15 },
    { time: "10:20", filter: 38, reserve: 14, bind: 14 },
    { time: "10:30", filter: 56, reserve: 21, bind: 19 },
    { time: "10:40", filter: 49, reserve: 18, bind: 17 },
  ],
};

export function simulatePlacement(
  request: SimulationRequest,
  sourceNodes: FabricNode[],
): Workload {
  const constrained = request.topologyMode !== "best-effort";
  const candidates = sourceNodes.map((node, index) => {
    if (!node.ready) return candidate(node.id, false, null, [], "Topology is unavailable in the fault drill.");
    if (request.vendor !== "any" && node.vendor !== request.vendor) {
      return candidate(node.id, false, null, [], `Vendor ${node.vendor} does not satisfy ${request.vendor}.`);
    }
    if (request.deviceCount > node.devices.filter((device) => device.health === "Healthy").length) {
      return candidate(node.id, false, null, [], "Insufficient healthy devices.");
    }
    if (constrained && request.deviceCount > 4) {
      return candidate(node.id, false, null, [], `No ${request.deviceCount}-device ${request.topologyMode} group exists; maximum group size is four.`);
    }
    if (request.minimumBandwidthGBps > node.bandwidthGBps) {
      return candidate(node.id, false, null, [], `${node.linkType} bandwidth is below the requested minimum.`);
    }
    const available = node.devices.filter((device) => device.reservedBy === null);
    const compatible = constrained
      ? [0, 1]
          .map((numa) => available.filter((device) => device.numa === numa))
          .find((group) => group.length >= request.deviceCount)
      : available;
    if (!compatible || compatible.length < request.deviceCount) {
      return candidate(node.id, false, null, [], "The reservation ledger has no compatible free device group.");
    }
    const selected = compatible.slice(0, request.deviceCount).map((device) => device.id);
    const score = 92 - index * 3 - Math.max(0, request.deviceCount - 2) * 2;
    return candidate(node.id, true, score, selected);
  });
  const selected = candidates
    .filter((item) => item.feasible)
    .sort((left, right) => (right.score ?? 0) - (left.score ?? 0))[0];
  const feasible = Boolean(selected);
  return {
    id: request.id,
    namespace: request.namespace,
    vendor: request.vendor,
    deviceCount: request.deviceCount,
    topologyMode: request.topologyMode,
    minimumBandwidthGBps: request.minimumBandwidthGBps,
    delivery: request.delivery,
    state: feasible ? "BOUND" : "UNSCHEDULABLE",
    createdAt: new Date().toISOString(),
    selectedNodeId: selected?.nodeId ?? null,
    policyName: `console-${request.topologyMode}`,
    podGroup: request.gangMembers > 1 ? `${request.id}-gang` : null,
    candidates,
    stages: feasible
      ? boundStages.map((stage) => ({ ...stage }))
      : [
          { name: "PreFilter", status: "complete", detail: "Simulation request normalized." },
          { name: "Filter", status: "failed", detail: "No node satisfies every request constraint." },
          { name: "Score", status: "pending", detail: "No feasible nodes to score." },
          { name: "Reserve", status: "pending", detail: "No reservation created." },
          { name: "Permit", status: "pending", detail: "Not reached." },
          { name: "PreBind", status: "pending", detail: "Not reached." },
        ],
  };
}
