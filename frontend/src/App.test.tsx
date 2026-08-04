import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import App from "./App";

afterEach(cleanup);

describe("Accelerator Fabric Console", () => {
  it("opens the fixture fabric inventory", () => {
    render(<App />);
    expect(screen.getByText("Scheduling cycle activity")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Fabric" }));
    expect(screen.getByRole("heading", { name: "H100-SXM fabric" })).toBeInTheDocument();
    expect(screen.getByText("Device attributes")).toBeInTheDocument();
    expect(screen.getByText("gpu0")).toBeInTheDocument();
  });

  it("runs a deterministic scheduling simulation", async () => {
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: "Simulator" }));
    fireEvent.change(screen.getByLabelText("Workload ID"), { target: { value: "console-simulation-001" } });
    fireEvent.click(screen.getByRole("button", { name: "Run scheduling cycle" }));
    expect(await screen.findByRole("heading", { name: "console-simulation-001" })).toBeInTheDocument();
    expect(screen.getByText("Filter and score")).toBeInTheDocument();
    expect(screen.getByText("gpu4")).toBeInTheDocument();
  });
});
