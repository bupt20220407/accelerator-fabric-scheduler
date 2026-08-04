import type { Vendor } from "../types";

export function vendorLabel(vendor: Vendor | "any") {
  if (vendor === "nvidia") return "NVIDIA";
  if (vendor === "huawei") return "Huawei";
  if (vendor === "amd") return "AMD";
  return "Any vendor";
}

export function timeLabel(value: string) {
  return new Intl.DateTimeFormat("en", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(value));
}

export function modeLabel(value: string) {
  return value
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}
