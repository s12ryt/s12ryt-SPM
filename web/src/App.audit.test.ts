import { mount, flushPromises, type VueWrapper } from "@vue/test-utils";
import { afterEach, expect, it, vi } from "vitest";
import App from "./App.vue";

const wrappers: VueWrapper[] = [];
const reply = (value: unknown) => ({
  ok: true,
  status: 200,
  json: async () => value,
});
const configuration = () => ({
  settings: {
    public: true,
    retentionDays: 7,
    rules: {
      cpu: 90,
      memory: 90,
      disk: 90,
      holdSeconds: 30,
      offlineSeconds: 15,
      intervalSeconds: 3,
    },
    notifications: {
      webhookEnabled: true,
      webhookURL: "https://private-endpoint.test/secret",
      telegramEnabled: false,
      telegramToken: "",
      telegramChat: "private-chat",
    },
  },
  database: {
    active: "sqlite:///private-path.db",
    pending: "",
    environment: false,
    error: "",
  },
});
function render() {
  const w = mount(App);
  wrappers.push(w);
  return w;
}
afterEach(() => {
  wrappers.splice(0).forEach((w) => w.unmount());
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it("clears administrative forms when the session expires on a public panel", async () => {
  vi.useFakeTimers();
  let admin = true;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) =>
      reply(
        path === "/api/session"
          ? { admin, public: true }
          : path === "/api/settings"
            ? configuration()
            : [],
      ),
    ),
  );
  const w = render();
  await flushPromises();
  await w
    .findAll("nav button")
    .find((b) => b.text().includes("系統設定"))!
    .trigger("click");
  await flushPromises();
  expect(w.text()).toContain("private-path.db");
  admin = false;
  await vi.advanceTimersByTimeAsync(3000);
  await flushPromises();
  expect(w.text()).not.toContain("private-path.db");
  expect(w.find('input[value="private-chat"]').exists()).toBe(false);
  expect(w.text()).not.toContain("儲存設定");
});

it("does not restore privileged monitoring data from a response arriving after logout", async () => {
  vi.useFakeTimers();
  let admin = true;
  let delay = false;
  let resolveNodes!: (value: unknown) => void;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path === "/api/logout") {
        admin = false;
        return reply({});
      }
      if (path === "/api/session") return reply({ admin, public: false });
      if (path === "/api/nodes" && delay)
        return new Promise((resolve) => {
          resolveNodes = resolve;
        });
      return reply([]);
    }),
  );
  const w = render();
  await flushPromises();
  delay = true;
  await vi.advanceTimersByTimeAsync(3000);
  await w.get('[aria-label="登出"]').trigger("click");
  await flushPromises();
  resolveNodes(
    reply([
      {
        id: "secret",
        name: "privileged-node",
        online: true,
        rules: null,
        latest: null,
        lastSeen: 0,
      },
    ]),
  );
  await flushPromises();
  expect(w.text()).toContain("登入監控工作空間");
  // Re-enter a public session to detect privileged data lingering in state.
  vi.mocked(fetch).mockImplementation(async (path: any) =>
    path === "/api/session"
      ? (reply({ admin: false, public: true }) as any)
      : new Promise(() => {}),
  );
  await vi.advanceTimersByTimeAsync(3000);
  await flushPromises();
  expect(w.text()).not.toContain("privileged-node");
});

it("times out stalled reads and aborts in-flight requests when unmounted", async () => {
  vi.useFakeTimers();
  let signal: AbortSignal | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn(
      (_path, init: RequestInit) =>
        new Promise((_resolve, reject) => {
          signal = init.signal ?? undefined;
          signal?.addEventListener("abort", () =>
            reject(new DOMException("aborted", "AbortError")),
          );
        }),
    ),
  );
  const w = render();
  await vi.advanceTimersByTimeAsync(10000);
  await flushPromises();
  expect(w.find('[role="alert"]').text()).toContain("逾時");
  await vi.advanceTimersByTimeAsync(3000);
  expect(signal?.aborted).toBe(false);
  w.unmount();
  wrappers.splice(wrappers.indexOf(w), 1);
  expect(signal?.aborted).toBe(true);
});

it("returns to the overview when another administrator deletes the selected host", async () => {
  vi.useFakeTimers();
  let exists = true;
  const host = {
    id: "removed",
    name: "待刪除主機",
    online: false,
    lastSeen: 0,
    latest: null,
    rules: null,
  };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path === "/api/session") return reply({ admin: false, public: true });
      if (path === "/api/nodes") return reply(exists ? [host] : []);
      if (path.includes("/history") && !exists)
        return {
          ok: false,
          status: 404,
          json: async () => ({ error: "找不到主機" }),
        };
      return reply([]);
    }),
  );
  const w = render();
  await flushPromises();
  await w.get('[data-testid="node-removed"]').trigger("click");
  await flushPromises();
  expect(w.text()).toContain("系統資訊");
  exists = false;
  await vi.advanceTimersByTimeAsync(3000);
  await flushPromises();
  expect(w.text()).toContain("開始監控你的第一台主機");
  expect(w.find('[role="alert"]').exists()).toBe(false);
});

it("does not switch hosts while the previous host's settings are loading", async () => {
  const rules = configuration().settings.rules;
  const hosts = [
    {
      id: "a",
      name: "主機甲",
      online: false,
      lastSeen: 0,
      latest: null,
      rules: { ...rules, intervalSeconds: 30 },
    },
    {
      id: "b",
      name: "主機乙",
      online: false,
      lastSeen: 0,
      latest: null,
      rules: { ...rules, intervalSeconds: 60 },
    },
  ];
  let resolveSettings!: (value: unknown) => void;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path === "/api/session") return reply({ admin: true, public: false });
      if (path === "/api/nodes") return reply(hosts);
      if (path === "/api/settings")
        return new Promise((resolve) => {
          resolveSettings = resolve;
        });
      return reply([]);
    }),
  );
  const w = render();
  await flushPromises();
  await w.get('[data-testid="node-a"]').trigger("click");
  await flushPromises();
  await w.findAll("nav button")[0]!.trigger("click");
  await w.get('[data-testid="node-b"]').trigger("click");
  resolveSettings(reply(configuration()));
  await flushPromises();
  // While busy the second action must not partially change the displayed host.
  expect(w.find('[data-testid="node-b"]').exists()).toBe(true);
  await w.get('[data-testid="node-b"]').trigger("click");
  await flushPromises();
  resolveSettings(reply(configuration()));
  await flushPromises();
  expect(w.get("h1").text()).toBe("主機乙");
  const interval = w
    .findAll("label")
    .find((label) => label.text().includes("採樣間隔"))!;
  expect((interval.get("input").element as HTMLInputElement).value).toBe("60");
});
