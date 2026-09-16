import { mount, flushPromises, type VueWrapper } from "@vue/test-utils";
import { afterEach, expect, it, vi } from "vitest";
import App from "./App.vue";

const wrappers: VueWrapper[] = [];
const reply = (value: unknown, status = 200) => ({
  ok: status >= 200 && status < 300,
  status,
  json: async () => value,
});

it("reports a readable message when a proxy replies with a non-JSON error page", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({
      ok: false,
      status: 502,
      json: async () => {
        throw new SyntaxError(
          "Unexpected token '<', \"<html><body>502</body></html>\" is not valid JSON",
        );
      },
    })),
  );
  const w = mount(App);
  wrappers.push(w);
  await flushPromises();
  const banner = w.get(".banner.error");
  expect(banner.text()).toContain("502");
  expect(banner.text()).not.toContain("Unexpected token");
});

function disableClipboard() {
  const target = Object.getPrototypeOf(navigator) as object;
  const original = Object.getOwnPropertyDescriptor(target, "clipboard");
  Object.defineProperty(target, "clipboard", {
    configurable: true,
    value: undefined,
  });
  return () => {
    if (original) Object.defineProperty(target, "clipboard", original);
    else Reflect.deleteProperty(target, "clipboard");
  };
}

it("copies the one-time token without the clipboard API on plain HTTP origins", async () => {
  const restore = disableClipboard();
  wrappers.push({ unmount: restore } as unknown as VueWrapper);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/api/session") return reply({ admin: true, public: false });
      if (path === "/api/nodes" && init?.method === "POST")
        return reply({ id: "n1", token: "one-time-token" });
      return reply([]);
    }),
  );
  const legacyCopy = vi.fn(() => true);
  document.execCommand = legacyCopy as unknown as typeof document.execCommand;
  const w = mount(App);
  wrappers.push(w);
  await flushPromises();
  await w
    .findAll("button")
    .find((b) => b.text().includes("新增主機"))!
    .trigger("click");
  await w.get('input[placeholder="例如：台北正式環境"]').setValue("測試主機");
  await w.get('[role="dialog"] form').trigger("submit");
  await flushPromises();
  await w
    .findAll("button")
    .find((b) => b.text().includes("複製 Token"))!
    .trigger("click");
  await flushPromises();
  expect(legacyCopy).toHaveBeenCalledWith("copy");
  expect(w.get(".banner.success").text()).toContain("Token 已複製");
});

it("shows a friendly message when every copy method is unavailable", async () => {
  const restore = disableClipboard();
  wrappers.push({ unmount: restore } as unknown as VueWrapper);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      if (path === "/api/session") return reply({ admin: true, public: false });
      if (path === "/api/nodes" && init?.method === "POST")
        return reply({ id: "n1", token: "one-time-token" });
      return reply([]);
    }),
  );
  document.execCommand = vi.fn(
    () => false,
  ) as unknown as typeof document.execCommand;
  const w = mount(App);
  wrappers.push(w);
  await flushPromises();
  await w
    .findAll("button")
    .find((b) => b.text().includes("新增主機"))!
    .trigger("click");
  await w.get('input[placeholder="例如：台北正式環境"]').setValue("測試主機");
  await w.get('[role="dialog"] form').trigger("submit");
  await flushPromises();
  await w
    .findAll("button")
    .find((b) => b.text().includes("複製 Token"))!
    .trigger("click");
  await flushPromises();
  expect(w.text()).toContain("剪貼簿");
  expect(w.text()).not.toContain("Cannot read");
});

afterEach(() => {
  wrappers.splice(0).forEach((w) => w.unmount());
  Reflect.deleteProperty(document, "execCommand");
  vi.unstubAllGlobals();
});
