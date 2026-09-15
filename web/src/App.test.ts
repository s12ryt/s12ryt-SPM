import { mount, flushPromises } from "@vue/test-utils";
import { afterEach, describe, it, expect, vi } from "vitest";
import App from "./App.vue";
const node = {
  id: "n1",
  name: "台北節點",
  online: true,
  lastSeen: Date.now(),
  rules: null,
  latest: {
    time: Date.now(),
    snapshot: {
      hostname: "vps",
      os: "linux",
      platform: "ubuntu",
      arch: "amd64",
      cores: 2,
      uptime: 86400,
      memoryTotal: 8589934592,
      memoryUsed: 4294967296,
      disks: [],
      networks: [],
    },
    metrics: { cpu: 25, memory: 50, disk: 30, rx: 1024, tx: 512 },
  },
};
function mock(admin = false, publicView = true) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: string) => ({
      ok: true,
      json: async () =>
        input === "/api/session"
          ? { admin, public: publicView }
          : input === "/api/nodes"
            ? [node]
            : input === "/api/alerts"
              ? []
              : [],
    })),
  );
}
afterEach(() => {
  vi.unstubAllGlobals();
});
describe("monitoring dashboard", () => {
  it("counts same-name hosts independently and uses recovery by node ID", async () => {
    mock();
    const base = vi.mocked(fetch).getMockImplementation()!;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: string, ...args: unknown[]) =>
        input === "/api/alerts"
          ? {
              ok: true,
              json: async () => [
                {
                  id: "3",
                  nodeId: "b",
                  nodeName: "同名",
                  kind: "cpu",
                  active: true,
                  time: 3,
                },
                {
                  id: "2",
                  nodeId: "a",
                  nodeName: "同名",
                  kind: "cpu",
                  active: true,
                  time: 2,
                },
              ],
            }
          : base(input, ...(args as [])),
      ),
    );
    const w = mount(App);
    await flushPromises();
    expect(w.findAll(".stat").at(-1)!.get("strong").text()).toBe("2筆");
    w.unmount();
  });
  it("shows disk history with the other resource trends", async () => {
    mock();
    const w = mount(App);
    await flushPromises();
    await w.get('[data-testid="node-n1"]').trigger("click");
    await flushPromises();
    expect(w.text()).toContain("磁碟使用率");
    w.unmount();
  });
  it("starts node overrides with the saved global rules", async () => {
    mock(true);
    const base = vi.mocked(fetch).getMockImplementation()!;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: string, ...args: unknown[]) =>
        input === "/api/settings"
          ? {
              ok: true,
              json: async () => ({
                settings: {
                  rules: {
                    cpu: 80,
                    memory: 85,
                    disk: 90,
                    holdSeconds: 20,
                    offlineSeconds: 60,
                    intervalSeconds: 15,
                  },
                },
              }),
            }
          : base(input, ...(args as [])),
      ),
    );
    const w = mount(App);
    await flushPromises();
    await w.get('[data-testid="node-n1"]').trigger("click");
    await flushPromises();
    await w.get('input[type="checkbox"]').setValue(true);
    expect(
      (w.findAll('input[type="number"]').at(-1)!.element as HTMLInputElement)
        .value,
    ).toBe("15");
    w.unmount();
  });
  it("requires login for private panel", async () => {
    mock(false, false);
    const w = mount(App);
    await flushPromises();
    expect(w.find('input[type="password"]').exists()).toBe(true);
    expect(w.text()).not.toContain("台北節點");
    w.unmount();
  });
  it("shows live metrics, filters and opens node history", async () => {
    mock();
    const w = mount(App);
    await flushPromises();
    expect(w.text()).toContain("台北節點");
    expect(w.text()).toContain("25.0");
    await w.get('input[placeholder="搜尋主機名稱、系統…"]').setValue("不存在");
    expect(w.text()).not.toContain("台北節點");
    await w.get('input[placeholder="搜尋主機名稱、系統…"]').setValue("");
    await w.get('[data-testid="node-n1"]').trigger("click");
    await flushPromises();
    expect(
      vi
        .mocked(fetch)
        .mock.calls.some((c) => String(c[0]).includes("/history?")),
    ).toBe(true);
    expect(w.text()).toContain("CPU 使用率");
    w.unmount();
  });
  it("hides administrative controls from public visitors", async () => {
    mock();
    const w = mount(App);
    await flushPromises();
    expect(w.text()).not.toContain("新增主機");
    w.unmount();
  });
  it("shows enrollment to administrators and surfaces network errors", async () => {
    mock(true);
    const w = mount(App);
    await flushPromises();
    expect(w.text()).toContain("新增主機");
    w.unmount();
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("連線失敗")));
    const failed = mount(App);
    await flushPromises();
    expect(failed.text()).toContain("連線失敗");
    failed.unmount();
  });
});
