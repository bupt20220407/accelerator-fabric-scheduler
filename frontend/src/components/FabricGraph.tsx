import {
  Background,
  Controls,
  ReactFlow,
  type Edge,
  type Node,
} from "@xyflow/react";
import { useEffect, useState } from "react";
import "@xyflow/react/dist/style.css";
import type { FabricNode } from "../types";

export function FabricGraph({ node }: { node: FabricNode }) {
  const [compact, setCompact] = useState(() =>
    window.matchMedia("(max-width: 560px)").matches,
  );

  useEffect(() => {
    const media = window.matchMedia("(max-width: 560px)");
    const update = () => setCompact(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  const graphNodes: Node[] = node.devices.map((device, index) => ({
    id: device.id,
    position: compact
      ? { x: 18 + (index % 2) * 145, y: 20 + Math.floor(index / 2) * 82 }
      : { x: 24 + (index % 4) * 152, y: 40 + Math.floor(index / 4) * 178 },
    data: {
      label: `${device.id}\nNUMA ${device.numa} · ${device.memoryGiB} GiB`,
    },
    className: `device-node ${device.reservedBy ? "device-reserved" : "device-free"}`,
  }));

  const graphEdges: Edge[] = [];
  for (let group = 0; group < 2; group += 1) {
    const start = group * 4;
    for (let left = start; left < start + 4; left += 1) {
      for (let right = left + 1; right < start + 4; right += 1) {
        graphEdges.push({
          id: `${left}-${right}`,
          source: node.devices[left].id,
          target: node.devices[right].id,
          className: "fabric-edge",
        });
      }
    }
  }
  graphEdges.push({
    id: "bridge",
    source: node.devices[3].id,
    target: node.devices[4].id,
    className: "bridge-edge",
    label: "PCIe · 64 GB/s · 2 hops",
  });

  return (
    <div className="fabric-canvas" aria-label={`${node.product} accelerator fabric`}>
      <ReactFlow
        key={`${node.id}-${compact ? "compact" : "wide"}`}
        nodes={graphNodes}
        edges={graphEdges}
        fitView
        fitViewOptions={{ padding: 0.12 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag
        zoomOnScroll={false}
        minZoom={0.55}
        maxZoom={1.2}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={20} size={1} color="#dce3df" />
        <Controls showInteractive={false} position="bottom-right" />
      </ReactFlow>
    </div>
  );
}
