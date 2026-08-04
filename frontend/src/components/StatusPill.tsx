interface Props {
  state: string;
}

export function StatusPill({ state }: Props) {
  const normalized = state.toLowerCase();
  const tone =
    normalized.includes("running") ||
    normalized.includes("bound") ||
    normalized.includes("prepared") ||
    normalized.includes("committed") ||
    normalized.includes("ready")
      ? "success"
      : normalized.includes("waiting") || normalized.includes("pending")
        ? "warning"
        : normalized.includes("unschedulable") ||
            normalized.includes("failed") ||
            normalized.includes("withdrawn")
          ? "danger"
          : "neutral";
  return <span className={`status-pill status-${tone}`}>{state}</span>;
}
