"use client";

import { create } from "zustand";
import type { Project } from "../types/project";
import { api } from "../api";
import { createLogger } from "../logger";

const logger = createLogger("project-list-store");

function sortProjects(projects: Project[]): Project[] {
  return [...projects].sort((a, b) => a.title.localeCompare(b.title));
}

interface ProjectListState {
  projects: Project[];
  loading: boolean;
  fetch: () => Promise<void>;
  setProjects: (projects: Project[]) => void;
  addProject: (p: Project) => void;
  updateProjectLocal: (id: string, updates: Partial<Project>) => void;
  removeProject: (id: string) => void;
  createProject: (title: string) => Promise<Project>;
  updateProject: (id: string, updates: { title?: string }) => Promise<Project>;
  deleteProject: (id: string) => Promise<void>;
  reset: () => void;
}

export const useProjectStore = create<ProjectListState>((set, get) => ({
  projects: [],
  loading: true,

  fetch: async () => {
    set({ loading: true });
    try {
      const res = await api.listProjects();
      set({ projects: sortProjects(res.projects), loading: false });
    } catch (err) {
      logger.error("fetch projects failed", err);
      set({ loading: false });
    }
  },

  setProjects: (projects) => set({ projects }),

  addProject: (p) =>
    set((s) => ({
      projects: sortProjects([...s.projects, p]),
    })),

  updateProjectLocal: (id, updates) =>
    set((s) => ({
      projects: s.projects.map((x) => (x.id === id ? { ...x, ...updates } : x)),
    })),

  removeProject: (id) =>
    set((s) => ({ projects: s.projects.filter((x) => x.id !== id) })),

  createProject: async (title) => {
    const p = await api.createProject({ title });
    get().addProject(p);
    return p;
  },

  updateProject: async (id, updates) => {
    const body = updates.title !== undefined ? { title: updates.title } : {};
    const p = await api.updateProject(id, body);
    get().updateProjectLocal(id, p);
    return p;
  },

  deleteProject: async (id) => {
    await api.deleteProject(id);
    get().removeProject(id);
  },

  reset: () => set({ projects: [], loading: true }),
}));
