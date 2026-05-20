import { describe, expect, it } from "vitest";
import type { Project } from "../types/project";
import { orderProjectsByPersonalPreference } from "./personal-tab-order";

function p(id: string, title: string): Project {
  const ts = "2020-01-01T00:00:00.000Z";
  return {
    id,
    workspace_id: "ws",
    title,
    description: null,
    icon: null,
    status: "planned",
    priority: "none",
    lead_type: null,
    lead_id: null,
    created_at: ts,
    updated_at: ts,
    issue_count: 0,
    done_count: 0,
    resource_count: 0,
  };
}

describe("orderProjectsByPersonalPreference", () => {
  it("sorts alphabetically by title when no saved order", () => {
    const projects = [p("a", "Zed"), p("b", "Alpha")];
    expect(orderProjectsByPersonalPreference(projects, null).map((x) => x.id)).toEqual(["b", "a"]);
  });

  it("applies saved id order and appends new projects by default sort", () => {
    const projects = [p("x", "B"), p("y", "A"), p("z", "C")];
    expect(
      orderProjectsByPersonalPreference(projects, ["z", "x"]).map((q) => q.id),
    ).toEqual(["z", "x", "y"]);
  });

  it("drops stale ids from storage", () => {
    const projects = [p("only", "One")];
    expect(
      orderProjectsByPersonalPreference(projects, ["gone", "only"]).map((q) => q.id),
    ).toEqual(["only"]);
  });
});
