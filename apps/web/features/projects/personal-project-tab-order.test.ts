import { describe, expect, it } from "vitest";
import type { Project } from "@/shared/types";
import { orderProjectsByPersonalPreference } from "./personal-project-tab-order";

function p(id: string, name: string, position: number): Project {
  const t = "2020-01-01T00:00:00.000Z";
  return {
    id,
    workspace_id: "ws",
    name,
    position,
    created_at: t,
    updated_at: t,
  };
}

describe("orderProjectsByPersonalPreference", () => {
  it("sorts by position then name when no saved order", () => {
    const projects = [p("a", "Zed", 2), p("b", "Alpha", 1)];
    expect(orderProjectsByPersonalPreference(projects, null).map((x) => x.id)).toEqual([
      "b",
      "a",
    ]);
  });

  it("applies saved id order and appends new projects by default sort", () => {
    const projects = [p("x", "B", 0), p("y", "A", 0), p("z", "C", 0)];
    expect(
      orderProjectsByPersonalPreference(projects, ["z", "x"]).map((q) => q.id),
    ).toEqual(["z", "x", "y"]);
  });

  it("drops stale ids from storage", () => {
    const projects = [p("only", "One", 0)];
    expect(
      orderProjectsByPersonalPreference(projects, ["gone", "only"]).map((q) => q.id),
    ).toEqual(["only"]);
  });
});
