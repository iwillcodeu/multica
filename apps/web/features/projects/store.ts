"use client";

import { create } from "zustand";
import type { Project } from "@/shared/types";
import { api } from "@/shared/api";
import { createLogger } from "@/shared/logger";

const logger = createLogger("project-store");

interface ProjectState {
  projects: Project[];
  loading: boolean;
  fetch: () => Promise<void>;
  setProjects: (projects: Project[]) => void;
  addProject: (p: Project) => void;
  updateProjectLocal: (id: string, updates: Partial<Project>) => void;
  removeProject: (id: string) => void;
  createProject: (name: string) => Promise<Project>;
  reset: () => void;
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  projects: [],
  loading: true,

  fetch: async () => {
    set({ loading: true });
    try {
      const res = await api.listProjects();
      const sorted = [...res.projects].sort(
        (a, b) => a.position - b.position || a.name.localeCompare(b.name),
      );
      set({ projects: sorted, loading: false });
    } catch (err) {
      logger.error("fetch projects failed", err);
      set({ loading: false });
    }
  },

  setProjects: (projects) => set({ projects }),

  addProject: (p) =>
    set((s) => ({
      projects: [...s.projects, p].sort(
        (a, b) => a.position - b.position || a.name.localeCompare(b.name),
      ),
    })),

  updateProjectLocal: (id, updates) =>
    set((s) => ({
      projects: s.projects.map((x) => (x.id === id ? { ...x, ...updates } : x)),
    })),

  removeProject: (id) =>
    set((s) => ({ projects: s.projects.filter((x) => x.id !== id) })),

  createProject: async (name) => {
    const p = await api.createProject({ name });
    get().addProject(p);
    return p;
  },

  reset: () => set({ projects: [], loading: true }),
}));
