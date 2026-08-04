import { lazy, Suspense, useMemo, useState, type FormEvent } from "react";
import {
  Activity,
  AlertTriangle,
  Boxes,
  CheckCircle2,
  CircleDot,
  Clock3,
  Cpu,
  Database,
  GitMerge,
  Layers3,
  Link2,
  LockKeyhole,
  MemoryStick,
  Network,
  Play,
  Server,
  ShieldCheck,
  Split,
  TimerReset,
  Waypoints,
  Zap,
} from "lucide-react";
import { modeLabel, timeLabel, vendorLabel } from "../lib/format";
import type {
  FabricNode,
  ReplaySnapshot,
  SimulationRequest,
  Vendor,
  Workload,
} from "../types";
import { StatusPill } from "../components/StatusPill";

const ActivityChart = lazy(() =>
  import("../components/ActivityChart").then((module) => ({ default: module.ActivityChart })),
);
const FabricGraph = lazy(() =>
  import("../components/FabricGraph").then((module) => ({ default: module.FabricGraph })),
);

function SectionHeading({ eyebrow, title, meta }: { eyebrow: string; title: string; meta?: string }) {
  return (
    <div className="section-heading">
      <div><span className="eyebrow">{eyebrow}</span><h2>{title}</h2></div>
      {meta && <span className="section-meta">{meta}</span>}
    </div>
  );
}

function WorkloadTable({ workloads, onSelect }: { workloads: Workload[]; onSelect: (id: string) => void }) {
  return (
    <div className="table-wrap">
      <table>
        <thead><tr><th>Workload</th><th>State</th><th>Policy</th><th>Request</th><th>Placement</th><th>Created</th></tr></thead>
        <tbody>
          {workloads.map((workload) => (
            <tr key={workload.id} tabIndex={0} onClick={() => onSelect(workload.id)} onKeyDown={(event) => { if (event.key === "Enter") onSelect(workload.id); }}>
              <td><strong>{workload.id}</strong><small>{workload.namespace}</small></td>
              <td><StatusPill state={workload.state} /></td>
              <td>{workload.policyName}</td>
              <td>{workload.deviceCount} · {vendorLabel(workload.vendor)}</td>
              <td>{workload.selectedNodeId?.replace("accelerator-fabric-", "") ?? "Unassigned"}</td>
              <td>{timeLabel(workload.createdAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function OverviewView({
  snapshot,
  fault,
  onSelectWorkload,
  onSelectNode,
}: {
  snapshot: ReplaySnapshot;
  fault: boolean;
  onSelectWorkload: (id: string) => void;
  onSelectNode: (id: string) => void;
}) {
  const readyNodes = snapshot.nodes.filter((node) => node.ready).length;
  const published = snapshot.nodes.filter((node) => node.ready).reduce((total, node) => total + node.devices.length, 0);
  const reserved = snapshot.nodes.reduce((total, node) => total + node.devices.filter((device) => device.reservedBy).length, 0);
  const active = snapshot.workloads.filter((workload) => workload.state !== "UNSCHEDULABLE").length;
  return (
    <div className="view-stack">
      {fault && <div className="alert-strip"><AlertTriangle size={17} /><span>Fault drill active: the AMD topology and its ResourceSlice pool are withdrawn.</span></div>}
      <section className="metric-grid" aria-label="Fabric metrics">
        <article className="metric-card"><span className="metric-icon"><Server size={19} /></span><div><span>Ready topologies</span><strong>{readyNodes}/3</strong><small>Fresh controller status</small></div></article>
        <article className="metric-card"><span className="metric-icon"><Cpu size={19} /></span><div><span>Published devices</span><strong>{published}</strong><small>Across node-local DRA pools</small></div></article>
        <article className="metric-card"><span className="metric-icon"><LockKeyhole size={19} /></span><div><span>Ledger reservations</span><strong>{reserved}</strong><small>Disjoint device identities</small></div></article>
        <article className="metric-card"><span className="metric-icon"><Activity size={19} /></span><div><span>Active workloads</span><strong>{active}</strong><small>Bound, running or waiting</small></div></article>
      </section>

      <div className="content-grid overview-grid">
        <section className="section-block">
          <SectionHeading eyebrow="Scheduler operations" title="Scheduling cycle activity" meta="Last 50 minutes" />
          <div className="chart-frame">
            <Suspense fallback={<div className="visual-loading">Loading operation chart</div>}><ActivityChart data={snapshot.activity} /></Suspense>
          </div>
        </section>
        <section className="section-block">
          <SectionHeading eyebrow="Control plane" title="Component state" meta={fault ? "degraded" : "healthy"} />
          <dl className="health-list">
            <div><dt><Waypoints size={16} />Scheduler</dt><dd>Ready</dd></div>
            <div><dt><TimerReset size={16} />Topology controller</dt><dd>Ready</dd></div>
            <div><dt><Network size={16} />Discovery agents</dt><dd>{fault ? "2 / 3" : "3 / 3"}</dd></div>
            <div><dt><Database size={16} />DRA drivers</dt><dd>{fault ? "2 / 3" : "3 / 3"}</dd></div>
          </dl>
        </section>
      </div>

      <section className="section-block">
        <SectionHeading eyebrow="Topology inventory" title="Accelerator nodes" meta={`${published} devices visible`} />
        <div className="node-grid">
          {snapshot.nodes.map((node) => {
            const used = node.devices.filter((device) => device.reservedBy).length;
            return (
              <button className={`node-card ${!node.ready ? "node-withdrawn" : ""}`} key={node.id} onClick={() => onSelectNode(node.id)}>
                <div className="node-title"><span className={`vendor-mark vendor-${node.vendor}`}>{vendorLabel(node.vendor).slice(0, 1)}</span><div><strong>{node.product}</strong><small>{node.shortName}</small></div><StatusPill state={node.ready ? "Ready" : "Withdrawn"} /></div>
                <div className="node-stats"><span><Cpu size={14} />{node.devices.length} devices</span><span><Link2 size={14} />{node.linkType} {node.bandwidthGBps} GB/s</span></div>
                <div className="usage-row"><span>{used} reserved</span><span>{node.ready ? node.devices.length - used : 0} available</span></div>
                <div className="usage-track"><span style={{ width: `${node.ready ? (used / node.devices.length) * 100 : 100}%` }} /></div>
              </button>
            );
          })}
        </div>
      </section>

      <section className="section-block">
        <SectionHeading eyebrow="Recent scheduling" title="Workload decisions" meta={`${snapshot.workloads.length} records`} />
        <WorkloadTable workloads={snapshot.workloads} onSelect={onSelectWorkload} />
      </section>
    </div>
  );
}

export function FabricView({ nodes, selectedNodeId, onNodeChange }: { nodes: FabricNode[]; selectedNodeId: string; onNodeChange: (id: string) => void }) {
  const node = nodes.find((item) => item.id === selectedNodeId) ?? nodes[0];
  if (!node) return <div className="empty-state">No topology fixture is available.</div>;
  const reserved = node.devices.filter((device) => device.reservedBy).length;
  return (
    <div className="view-stack">
      <div className="view-header">
        <div><span className="eyebrow">Immutable topology snapshot</span><h2>{node.product} fabric</h2><p>{node.resourceName} · observed generation 1</p></div>
        <label className="select-field"><span>Node topology</span><select aria-label="Node topology" value={node.id} onChange={(event) => onNodeChange(event.target.value)}>{nodes.map((item) => <option key={item.id} value={item.id}>{item.product} · {item.shortName}</option>)}</select></label>
      </div>
      {!node.ready && <div className="alert-strip"><AlertTriangle size={17} /><span>This topology is stale and excluded from FailClosed scheduling.</span></div>}
      <div className="content-grid fabric-layout">
        <section className="section-block fabric-block">
          <SectionHeading eyebrow="Device graph" title={`${node.linkType} connectivity`} meta="Two four-device cliques" />
          <Suspense fallback={<div className="fabric-canvas visual-loading">Loading fabric graph</div>}><FabricGraph node={node} /></Suspense>
          <div className="graph-legend"><span><i className="legend-free" />Available</span><span><i className="legend-reserved" />Reserved</span><span><i className="legend-fabric" />{node.linkType}</span><span><i className="legend-bridge" />PCIe bridge</span></div>
        </section>
        <aside className="section-block">
          <SectionHeading eyebrow="ResourceSlice" title="Pool contract" meta={node.ready ? "published" : "withdrawn"} />
          <dl className="allocation-list">
            <div><dt>Vendor</dt><dd>{vendorLabel(node.vendor)}</dd></div>
            <div><dt>Product</dt><dd>{node.product}</dd></div>
            <div><dt>Pool</dt><dd>{node.id}</dd></div>
            <div><dt>Fabric</dt><dd>{node.linkType} · {node.bandwidthGBps} GB/s</dd></div>
            <div><dt>Devices</dt><dd>{node.ready ? node.devices.length : 0} published</dd></div>
            <div><dt>Ledger</dt><dd>{reserved} reserved</dd></div>
            <div><dt>Heartbeat</dt><dd>{timeLabel(node.heartbeat)}</dd></div>
          </dl>
        </aside>
      </div>
      <section className="section-block">
        <SectionHeading eyebrow="Inventory" title="Device attributes" meta={`${node.devices.length} fixture devices`} />
        <div className="device-grid">
          {node.devices.map((device) => <article className="device-card" key={device.id}><div><strong>{device.id}</strong><StatusPill state={device.reservedBy ? "Reserved" : device.health} /></div><span>NUMA {device.numa} · root {device.pcieRoot}</span><span>{device.fabricGroup} · {device.memoryGiB} GiB</span><small>{device.reservedBy ? `Owner: ${device.reservedBy}` : "Reservation ledger: free"}</small></article>)}
        </div>
      </section>
    </div>
  );
}

export function DecisionsView({ workloads, nodes, selectedWorkloadId, onWorkloadChange }: { workloads: Workload[]; nodes: FabricNode[]; selectedWorkloadId: string; onWorkloadChange: (id: string) => void }) {
  const workload = workloads.find((item) => item.id === selectedWorkloadId) ?? workloads[0];
  if (!workload) return <div className="empty-state">No scheduling decision is available.</div>;
  return (
    <div className="view-stack">
      <div className="view-header decision-header">
        <div><span className="eyebrow">TopologyFit decision</span><h2>{workload.id}</h2><div className="inline-meta"><StatusPill state={workload.state} /><span>{workload.namespace}</span><span>{workload.delivery}</span></div></div>
        <label className="select-field"><span>Workload record</span><select aria-label="Workload record" value={workload.id} onChange={(event) => onWorkloadChange(event.target.value)}>{workloads.map((item) => <option key={item.id} value={item.id}>{item.id}</option>)}</select></label>
      </div>
      {workload.state === "UNSCHEDULABLE" && <div className="alert-strip"><AlertTriangle size={17} /><span>No node passed Filter. Scalar capacity alone cannot satisfy this topology request.</span></div>}
      <div className="content-grid decision-layout">
        <section className="section-block">
          <SectionHeading eyebrow="Node evaluation" title="Filter and score" meta={`${workload.candidates.filter((item) => item.feasible).length} feasible`} />
          <div className="candidate-grid">
            {workload.candidates.map((item) => {
              const node = nodes.find((candidateNode) => candidateNode.id === item.nodeId);
              return <article key={item.nodeId} className={`candidate-card ${item.nodeId === workload.selectedNodeId ? "candidate-selected" : ""}`}><div className="candidate-title"><div>{item.feasible ? <CheckCircle2 size={17} /> : <CircleDot size={17} />}<strong>{node?.product ?? item.nodeId}</strong></div>{item.score !== null && <b>{item.score}</b>}</div><span>{node?.shortName}</span>{item.feasible ? <><div className="score-bars"><div><span>Locality</span><i><b style={{ width: `${item.locality}%` }} /></i><em>{item.locality}</em></div><div><span>Fabric</span><i><b style={{ width: `${item.fabric}%` }} /></i><em>{item.fabric}</em></div><div><span>Fragment</span><i><b style={{ width: `${item.fragmentation}%` }} /></i><em>{item.fragmentation}</em></div><div><span>Headroom</span><i><b style={{ width: `${item.headroom}%` }} /></i><em>{item.headroom}</em></div></div><div className="device-selection">{item.selectedDevices.map((device) => <code key={device}>{device}</code>)}</div></> : <p>{item.reason}</p>}</article>;
            })}
          </div>
        </section>
        <aside className="section-block">
          <SectionHeading eyebrow="Placement policy" title={workload.policyName} meta="FailClosed" />
          <dl className="allocation-list">
            <div><dt><Cpu size={15} />Request</dt><dd>{workload.deviceCount} · {vendorLabel(workload.vendor)}</dd></div>
            <div><dt><Split size={15} />Topology</dt><dd>{modeLabel(workload.topologyMode)}</dd></div>
            <div><dt><Link2 size={15} />Minimum link</dt><dd>{workload.minimumBandwidthGBps || "None"} GB/s</dd></div>
            <div><dt><Database size={15} />Delivery</dt><dd>{workload.delivery}</dd></div>
            <div><dt><GitMerge size={15} />PodGroup</dt><dd>{workload.podGroup ?? "None"}</dd></div>
          </dl>
        </aside>
      </div>
      <section className="section-block">
        <SectionHeading eyebrow="Scheduling framework" title="Cycle trace" meta="Reserve ownership boundary" />
        <ol className="pipeline">
          {workload.stages.map((stage, index) => <li key={stage.name} className={`pipeline-${stage.status}`}><span>{index + 1}</span><div><strong>{stage.name}</strong><p>{stage.detail}</p></div></li>)}
        </ol>
      </section>
    </div>
  );
}

export function DRAView({ snapshot, fault }: { snapshot: ReplaySnapshot; fault: boolean }) {
  const gang = snapshot.podGroups[0];
  return (
    <div className="view-stack">
      {fault && <div className="alert-strip"><AlertTriangle size={17} /><span>AMD pool withdrawal prevents new allocations; existing prepared ownership remains visible for recovery analysis.</span></div>}
      <section className="section-block">
        <SectionHeading eyebrow="resource.k8s.io/v1" title="Node-local ResourceSlices" meta={`${fault ? 16 : 24} devices published`} />
        <div className="slice-grid">
          {snapshot.nodes.map((node) => <article className="slice-card" key={node.id}><div><span className={`vendor-mark vendor-${node.vendor}`}>{vendorLabel(node.vendor).slice(0, 1)}</span><div><strong>{node.id}</strong><small>{node.product}</small></div></div><StatusPill state={node.ready ? "Ready" : "Withdrawn"} /><dl><div><dt>Pool generation</dt><dd>1</dd></div><div><dt>Published</dt><dd>{node.ready ? 8 : 0}</dd></div><div><dt>Attributed</dt><dd>8 fields</dd></div></dl></article>)}
        </div>
      </section>
      <div className="content-grid dra-layout">
        <section className="section-block">
          <SectionHeading eyebrow="Authoritative allocation" title="ResourceClaims" meta={`${snapshot.claims.length} active`} />
          <div className="table-wrap"><table><thead><tr><th>Claim</th><th>State</th><th>Pool</th><th>Devices</th><th>CDI</th></tr></thead><tbody>{snapshot.claims.map((claim) => <tr key={claim.id}><td><strong>{claim.id}</strong><small>{claim.workloadId}</small></td><td><StatusPill state={claim.state} /></td><td>{claim.pool.replace("accelerator-fabric-", "")}</td><td><code>{claim.devices.join(", ")}</code></td><td>{claim.cdiVerified ? "Verified" : "Pending"}</td></tr>)}</tbody></table></div>
        </section>
        <section className="section-block">
          <SectionHeading eyebrow="Kubelet handoff" title="Claim identity chain" meta="exact match" />
          <ol className="handoff-list">
            <li><span><Database size={17} /></span><div><strong>ResourceClaim allocation</strong><p>Pool and device IDs become authoritative.</p></div></li>
            <li><span><LockKeyhole size={17} /></span><div><strong>NodePrepareResources</strong><p>Driver persists prepared ownership.</p></div></li>
            <li><span><Boxes size={17} /></span><div><strong>CDI injection</strong><p>Container receives the same device identities.</p></div></li>
            <li><span><ShieldCheck size={17} /></span><div><strong>Workload verification</strong><p>Claim, Prepare and CDI IDs agree.</p></div></li>
          </ol>
        </section>
      </div>
      {gang && <section className="section-block"><SectionHeading eyebrow="Coscheduling" title={`PodGroup · ${gang.id}`} meta={`${gang.minimumMembers}/${gang.minimumMembers} committed`} /><div className="gang-flow">{gang.members.map((member, index) => <div key={member.id} className="gang-member"><span className={`vendor-mark vendor-${member.vendor}`}>{vendorLabel(member.vendor).slice(0, 1)}</span><div><strong>{member.id}</strong><small>{member.nodeId?.replace("accelerator-fabric-", "")}</small></div><StatusPill state={member.state} />{index < gang.members.length - 1 && <i />}</div>)}<div className="commit-barrier"><GitMerge size={20} /><div><strong>Permit barrier released</strong><small>All members nominated before binding</small></div><StatusPill state={gang.phase} /></div></div></section>}
    </div>
  );
}

export function SimulatorView({ nodes, onRun }: { nodes: FabricNode[]; onRun: (request: SimulationRequest) => void }) {
  const [form, setForm] = useState<SimulationRequest>(() => ({
    id: `demo-placement-${String(Date.now()).slice(-5)}`,
    namespace: "demo-lab",
    vendor: "nvidia",
    deviceCount: 4,
    topologyMode: "fabric-clique",
    minimumBandwidthGBps: 300,
    delivery: "DRA",
    gangMembers: 1,
  }));
  const selectedNode = useMemo(() => nodes.find((node) => form.vendor === "any" || node.vendor === form.vendor), [form.vendor, nodes]);
  const update = <K extends keyof SimulationRequest>(key: K, value: SimulationRequest[K]) => setForm((current) => ({ ...current, [key]: value }));
  const submit = (event: FormEvent) => { event.preventDefault(); onRun(form); };
  return (
    <div className="simulator-layout">
      <form className="section-block" onSubmit={submit}>
        <SectionHeading eyebrow="Deterministic replay" title="Run a scheduling cycle" meta="No cluster mutation" />
        <div className="form-grid">
          <label><span>Workload ID</span><input required pattern="[a-z0-9]([-a-z0-9.]*[a-z0-9])?" value={form.id} onChange={(event) => update("id", event.target.value)} /></label>
          <label><span>Namespace</span><input required value={form.namespace} onChange={(event) => update("namespace", event.target.value)} /></label>
          <label><span>Vendor</span><select value={form.vendor} onChange={(event) => update("vendor", event.target.value as Vendor | "any")}><option value="any">Any vendor</option><option value="nvidia">NVIDIA</option><option value="huawei">Huawei</option><option value="amd">AMD</option></select></label>
          <label><span>Device count</span><input type="number" min="1" max="8" value={form.deviceCount} onChange={(event) => update("deviceCount", Number(event.target.value))} /></label>
          <label><span>Topology mode</span><select value={form.topologyMode} onChange={(event) => update("topologyMode", event.target.value as SimulationRequest["topologyMode"])}><option value="single-numa">Single NUMA</option><option value="same-pcie-root">Same PCIe root</option><option value="fabric-clique">Fabric clique</option><option value="fabric-connected">Fabric connected</option><option value="best-effort">Best effort</option></select></label>
          <label><span>Minimum bandwidth (GB/s)</span><input type="number" min="0" max="500" value={form.minimumBandwidthGBps} onChange={(event) => update("minimumBandwidthGBps", Number(event.target.value))} /></label>
          <label><span>Delivery path</span><select value={form.delivery} onChange={(event) => update("delivery", event.target.value as SimulationRequest["delivery"])}><option value="DRA">DRA ResourceClaim</option><option value="Extended resource">Extended resource</option></select></label>
          <label><span>Gang members</span><input type="number" min="1" max="8" value={form.gangMembers} onChange={(event) => update("gangMembers", Number(event.target.value))} /></label>
        </div>
        <div className="submit-actions"><button className="button button-primary" type="submit"><Play size={16} />Run scheduling cycle</button></div>
      </form>
      <aside className="simulation-summary">
        <span className="eyebrow">Evaluation envelope</span><h2>{form.id}</h2><p>The simulator applies the same fixture limits and deterministic selection rules demonstrated by the project tests.</p>
        <dl><div><dt>Candidate product</dt><dd>{selectedNode?.product ?? "Highest score"}</dd></div><div><dt>Request</dt><dd>{form.deviceCount} · {vendorLabel(form.vendor)}</dd></div><div><dt>Topology</dt><dd>{modeLabel(form.topologyMode)}</dd></div><div><dt>Link threshold</dt><dd>{form.minimumBandwidthGBps || "None"} GB/s</dd></div><div><dt>Admission</dt><dd>{form.gangMembers > 1 ? `${form.gangMembers}-member PodGroup` : "Single Pod"}</dd></div></dl>
        <div className="policy-check"><CheckCircle2 size={16} />FailClosed topology availability</div><div className="policy-check"><CheckCircle2 size={16} />Atomic reservation ledger</div><div className="policy-check"><CheckCircle2 size={16} />Deterministic device ordering</div>
      </aside>
    </div>
  );
}
