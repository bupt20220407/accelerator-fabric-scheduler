import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { ActivityPoint } from "../types";

export function ActivityChart({ data }: { data: ActivityPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={data} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
        <CartesianGrid stroke="#e6ebe8" vertical={false} />
        <XAxis dataKey="time" axisLine={false} tickLine={false} tick={{ fill: "#68736d", fontSize: 10 }} />
        <YAxis axisLine={false} tickLine={false} tick={{ fill: "#68736d", fontSize: 10 }} />
        <Tooltip contentStyle={{ border: "1px solid #dce3df", borderRadius: 5, fontSize: 11 }} />
        <Legend wrapperStyle={{ fontSize: 10, paddingTop: 8 }} />
        <Line type="monotone" dataKey="filter" name="Filter" stroke="#2779b8" strokeWidth={2} dot={false} />
        <Line type="monotone" dataKey="reserve" name="Reserve" stroke="#b86d13" strokeWidth={2} dot={false} />
        <Line type="monotone" dataKey="bind" name="Bind" stroke="#12805c" strokeWidth={2} dot={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}
