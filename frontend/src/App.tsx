import { useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  Boxes,
  Cpu,
  GitBranch,
  LayoutDashboard,
  Network,
  RefreshCw,
  RotateCcw,
  TestTube2,
  X,
} from "lucide-react";
import { replaySnapshot, simulatePlacement } from "./data/replay";
import { timeLabel } from "./lib/format";
import type { ReplayMode, ReplaySnapshot, SimulationRequest, ViewId } from "./types";
import {
  DRAView,
  DecisionsView,
  FabricView,
  OverviewView,
  SimulatorView,
} from "./views/ConsoleViews";

const navigation: Array<{ id: ViewId; label: string; icon: typeof Activity }> = [
  { id: "overview", label: "Overview", icon: LayoutDashboard },
  { id: "fabric", label: "Fabric", icon: Network },
  { id: "decisions", label: "Decisions", icon: GitBranch },
  { id: "dra", label: "DRA & Gang", icon: Boxes },
  { id: "simulator", label: "Simulator", icon: TestTube2 },
];

const viewTitles: Record<ViewId, { title: string; subtitle: string }> = {
  overview: { title: "Fabric operations", subtitle: "Topology-aware accelerator placement" },
  fabric: { title: "Device fabric", subtitle: "NUMA, PCIe and accelerator-link topology" },
  decisions: { title: "Scheduling decisions", subtitle: "Filter, score and reservation evidence" },
  dra: { title: "DRA and gang", subtitle: "Authoritative identity and all-or-nothing admission" },
  simulator: { title: "Policy simulator", subtitle: "Deterministic fixture-based scheduling cycle" },
};

export default function App() {
  const [view, setView] = useState<ViewId>("overview");
  const [mode, setMode] = useState<ReplayMode>("baseline");
  const [snapshot, setSnapshot] = useState<ReplaySnapshot>(replaySnapshot);
  const [selectedNodeId, setSelectedNodeId] = useState(snapshot.nodes[0]?.id ?? "");
  const [selectedWorkloadId, setSelectedWorkloadId] = useState(snapshot.workloads[0]?.id ?? "");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const effectiveSnapshot = useMemo<ReplaySnapshot>(() => {
    if (mode === "baseline") return snapshot;
    return {
      ...snapshot,
      nodes: snapshot.nodes.map((node) =>
        node.vendor === "amd" ? { ...node, ready: false } : node,
      ),
    };
  }, [mode, snapshot]);

  const openWorkload = (id: string) => {
    setSelectedWorkloadId(id);
    setView("decisions");
  };

  const openNode = (id: string) => {
    setSelectedNodeId(id);
    setView("fabric");
  };

  const switchMode = (next: ReplayMode) => {
    setMode(next);
    setMessage(next === "fault" ? "AMD topology withdrawal injected." : "Baseline fixture restored.");
    setError(null);
  };

  const refresh = () => {
    setSnapshot((current) => ({ ...current, generatedAt: new Date().toISOString() }));
    setMessage("Replay snapshot refreshed.");
  };

  const runSimulation = (request: SimulationRequest) => {
    if (snapshot.workloads.some((workload) => workload.id === request.id)) {
      setError(`Workload ${request.id} already exists in this replay.`);
      return;
    }
    const workload = simulatePlacement(request, effectiveSnapshot.nodes);
    const selectedCandidate = workload.candidates.find((candidate) => candidate.nodeId === workload.selectedNodeId);
    setSnapshot((current) => ({
      ...current,
      generatedAt: new Date().toISOString(),
      workloads: [workload, ...current.workloads],
      nodes: current.nodes.map((node) =>
        node.id !== workload.selectedNodeId
          ? node
          : {
              ...node,
              devices: node.devices.map((device) =>
                selectedCandidate?.selectedDevices.includes(device.id)
                  ? { ...device, reservedBy: workload.id }
                  : device,
              ),
            },
      ),
    }));
    setSelectedWorkloadId(workload.id);
    setView("decisions");
    setError(null);
    setMessage(
      workload.selectedNodeId
        ? `${workload.id} completed the replay scheduling cycle.`
        : `${workload.id} was rejected by every candidate node.`,
    );
  };

  const page = (() => {
    switch (view) {
      case "overview":
        return <OverviewView snapshot={effectiveSnapshot} fault={mode === "fault"} onSelectWorkload={openWorkload} onSelectNode={openNode} />;
      case "fabric":
        return <FabricView nodes={effectiveSnapshot.nodes} selectedNodeId={selectedNodeId} onNodeChange={setSelectedNodeId} />;
      case "decisions":
        return <DecisionsView workloads={effectiveSnapshot.workloads} nodes={effectiveSnapshot.nodes} selectedWorkloadId={selectedWorkloadId} onWorkloadChange={setSelectedWorkloadId} />;
      case "dra":
        return <DRAView snapshot={effectiveSnapshot} fault={mode === "fault"} />;
      case "simulator":
        return <SimulatorView nodes={effectiveSnapshot.nodes} onRun={runSimulation} />;
    }
  })();

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <button className="brand" onClick={() => setView("overview")} aria-label="Accelerator Fabric Console home">
          <span className="brand-mark"><Cpu size={21} /></span>
          <span><strong>Accelerator Fabric</strong><small>Scheduler Console</small></span>
        </button>
        <nav aria-label="Primary navigation">
          {navigation.map((item) => {
            const Icon = item.icon;
            return <button key={item.id} className={view === item.id ? "nav-active" : ""} onClick={() => setView(item.id)} aria-current={view === item.id ? "page" : undefined}><Icon size={18} /><span>{item.label}</span></button>;
          })}
        </nav>
        <div className="sidebar-footer"><span className="runtime-dot" /><div><strong>v0.1.0</strong><small>Fixture replay</small></div></div>
      </aside>

      <main className="main-content">
        <header className="topbar">
          <div className="title-group"><span className="mobile-brand"><Cpu size={18} />Accelerator Fabric</span><h1>{viewTitles[view].title}</h1><p>{viewTitles[view].subtitle}</p></div>
          <div className="topbar-actions">
            <div className="mode-control" aria-label="Replay scenario">
              <button className={mode === "baseline" ? "mode-active" : ""} onClick={() => switchMode("baseline")}><RotateCcw size={14} />Baseline</button>
              <button className={mode === "fault" ? "mode-active mode-fault" : ""} onClick={() => switchMode("fault")}><AlertTriangle size={14} />Fault drill</button>
            </div>
            <button className="icon-button" onClick={refresh} title="Refresh replay" aria-label="Refresh replay"><RefreshCw size={18} /></button>
          </div>
        </header>
        <div className="sync-row"><span><span className={`sync-dot sync-${mode}`} />{mode === "fault" ? "AMD pool withdrawal" : "W10 deterministic fixture"}</span><span>Snapshot {timeLabel(snapshot.generatedAt)}</span></div>
        {(message || error) && <div className={`toast ${error ? "toast-error" : "toast-success"}`} role="status"><span>{error ?? message}</span><button onClick={() => { setMessage(null); setError(null); }} aria-label="Dismiss notification"><X size={15} /></button></div>}
        <div className="page-content">{page}</div>
      </main>
    </div>
  );
}
