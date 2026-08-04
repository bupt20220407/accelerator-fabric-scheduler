import { describe, expect, it } from "vitest";
import { replaySnapshot, simulatePlacement } from "./replay";

describe("fixture placement simulation", () => {
  it("keeps a constrained Huawei request inside one free clique", () => {
    const workload = simulatePlacement(
      {
        id: "huawei-clique-test",
        namespace: "test",
        vendor: "huawei",
        deviceCount: 4,
        topologyMode: "fabric-clique",
        minimumBandwidthGBps: 300,
        delivery: "DRA",
        gangMembers: 1,
      },
      replaySnapshot.nodes,
    );

    expect(workload.selectedNodeId).toBe("accelerator-fabric-worker2");
    expect(workload.candidates.find((item) => item.feasible)?.selectedDevices).toEqual([
      "ascend4",
      "ascend5",
      "ascend6",
      "ascend7",
    ]);
  });

  it("rejects a five-device fabric clique request", () => {
    const workload = simulatePlacement(
      {
        id: "five-device-test",
        namespace: "test",
        vendor: "nvidia",
        deviceCount: 5,
        topologyMode: "fabric-clique",
        minimumBandwidthGBps: 300,
        delivery: "Extended resource",
        gangMembers: 1,
      },
      replaySnapshot.nodes,
    );

    expect(workload.state).toBe("UNSCHEDULABLE");
    expect(workload.selectedNodeId).toBeNull();
  });
});
