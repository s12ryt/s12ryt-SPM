<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import {
  Activity,
  Server,
  Search,
  Plus,
  ArrowUpRight,
  ArrowDown,
  ArrowUp,
  Settings as SettingsIcon,
  Bell,
  LayoutDashboard,
  LogOut,
  Shield,
  ArrowLeft,
  X,
  Copy,
  RefreshCw,
} from "@lucide/vue";
import type { Node, Alert, Point, Config, Rules } from "./types";
import RuleFields from "./RuleFields.vue";
import Trend from "./Trend.vue";
const session = ref({ admin: false, public: false }),
  ready = ref(false),
  error = ref(""),
  notice = ref(""),
  busy = ref(false);
const nodes = ref<Node[]>([]),
  alerts = ref<Alert[]>([]),
  search = ref(""),
  tab = ref("overview"),
  selected = ref(""),
  points = ref<Point[]>([]),
  hours = ref(1),
  config = ref<Config | null>(null);
const username = ref(""),
  password = ref(""),
  loginOpen = ref(false),
  enroll = ref(false),
  name = ref(""),
  credential = ref<{ id: string; token: string } | null>(null),
  databaseURL = ref(""),
  override = ref(false),
  editName = ref(""),
  editRules = ref<Rules>({
    cpu: 90,
    memory: 90,
    disk: 90,
    holdSeconds: 30,
    offlineSeconds: 15,
    intervalSeconds: 3,
  }),
  deleteConfirm = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;
let stopped = false;
let historyVersion = 0;
let authVersion = 0;
let refreshVersion = 0;
const requests = new Set<AbortController>();
class StaleRequest extends Error {}
function clearAccess(publicView = false) {
  authVersion++;
  historyVersion++;
  session.value = { admin: false, public: publicView };
  config.value = null;
  credential.value = null;
  databaseURL.value = "";
  password.value = "";
  enroll.value = false;
  selected.value = "";
  tab.value = "overview";
  nodes.value = [];
  alerts.value = [];
  points.value = [];
}
const visible = computed(() =>
  nodes.value.filter((n) =>
    (
      n.name +
      " " +
      n.latest?.snapshot.platform +
      " " +
      n.latest?.snapshot.hostname
    )
      .toLowerCase()
      .includes(search.value.toLowerCase()),
  ),
);
const current = computed(() =>
  nodes.value.find((n) => n.id === selected.value),
);
const online = computed(() => nodes.value.filter((n) => n.online));
const average = computed(() => {
  const values = online.value.flatMap((n) =>
    n.latest?.metrics.cpu == null ? [] : [n.latest.metrics.cpu],
  );
  return values.length
    ? values.reduce((a, b) => a + b, 0) / values.length
    : null;
});
const totalRX = computed(() =>
  online.value.reduce((v, n) => v + (n.latest?.metrics.rx ?? 0), 0),
);
const activeAlerts = computed(() => {
  const seen = new Set<string>();
  return alerts.value.filter((a) => {
    const key = a.nodeId + ":" + a.kind;
    if (seen.has(key)) return false;
    seen.add(key);
    return a.active;
  });
});
const percent = (v: number | null | undefined) =>
  v == null ? "—" : v.toFixed(1);
const bytes = (v: number | null | undefined) => {
  if (v == null) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  while (v >= 1024 && i < 4) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i ? 1 : 0)} ${units[i]}`;
};
const date = (v: number) =>
  new Date(v).toLocaleString("zh-TW", { hour12: false });
async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const version = authVersion;
  const controller = new AbortController();
  requests.add(controller);
  const timeout = setTimeout(() => controller.abort(), 10000);
  try {
    const r = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: { "Content-Type": "application/json", "X-SPM-CSRF": "1" },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
    });
    const data = await r.json();
    if (stopped || version !== authVersion) throw new StaleRequest();
    if (r.status === 401 && path !== "/api/login") clearAccess();
    if (!r.ok) throw new Error(data.error || `請求失敗（${r.status}）`);
    return data;
  } catch (e) {
    if (stopped || (version !== authVersion && e instanceof StaleRequest))
      throw new StaleRequest();
    if (controller.signal.aborted) throw new Error("連線逾時，請稍後重試");
    throw e;
  } finally {
    clearTimeout(timeout);
    requests.delete(controller);
  }
}
async function action(fn: () => Promise<void>) {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  notice.value = "";
  try {
    await fn();
  } catch (e) {
    if (!(e instanceof StaleRequest))
      error.value = e instanceof Error ? e.message : "操作失敗";
  } finally {
    busy.value = false;
  }
}
async function history() {
  const id = selected.value,
    version = ++historyVersion;
  if (!id) return;
  const to = Date.now();
  const result = await api<Point[]>(
    `/api/nodes/${id}/history?from=${to - hours.value * 3600000}&to=${to}&limit=180`,
  );
  if (version === historyVersion && id === selected.value)
    points.value = result ?? [];
}
async function refresh() {
  const refreshID = ++refreshVersion;
  try {
    const next = await api<{ admin: boolean; public: boolean }>("/api/session");
    if (refreshID !== refreshVersion) throw new StaleRequest();
    if (session.value.admin && !next.admin) clearAccess(next.public);
    session.value = next;
    const version = authVersion;
    if (session.value.admin || session.value.public) {
      const [n, a] = await Promise.all([
        api<Node[]>("/api/nodes"),
        api<Alert[]>("/api/alerts"),
      ]);
      if (refreshID !== refreshVersion || version !== authVersion || stopped)
        throw new StaleRequest();
      nodes.value = n ?? [];
      alerts.value = a ?? [];
      if (
        selected.value &&
        !nodes.value.some((node) => node.id === selected.value)
      ) {
        selected.value = "";
        points.value = [];
        historyVersion++;
      }
      if (selected.value) await history();
    } else {
      nodes.value = [];
      alerts.value = [];
      points.value = [];
      selected.value = "";
      config.value = null;
      tab.value = "overview";
    }
    if (refreshID !== refreshVersion) throw new StaleRequest();
    ready.value = true;
  } catch (e) {
    if (refreshID !== refreshVersion) throw new StaleRequest();
    throw e;
  }
}
async function poll() {
  try {
    await refresh();
    error.value = "";
  } catch (e) {
    if (!(e instanceof StaleRequest))
      error.value = e instanceof Error ? e.message : "讀取失敗";
    ready.value = true;
  } finally {
    if (!stopped) timer = setTimeout(poll, 3000);
  }
}
onMounted(poll);
onUnmounted(() => {
  stopped = true;
  clearTimeout(timer);
  historyVersion++;
  for (const controller of requests) controller.abort();
});
async function login() {
  await action(async () => {
    authVersion++;
    await api("/api/login", "POST", {
      username: username.value,
      password: password.value,
    });
    password.value = "";
    loginOpen.value = false;
    await refresh();
  });
}
async function logout() {
  await action(async () => {
    clearAccess();
    await api("/api/logout", "POST");
    config.value = null;
    credential.value = null;
    enroll.value = false;
    selected.value = "";
    tab.value = "overview";
    await refresh();
  });
}
async function openNode(n: Node) {
  if (busy.value) return;
  selected.value = n.id;
  points.value = [];
  override.value = !!n.rules;
  editName.value = n.name;
  deleteConfirm.value = false;
  await action(async () => {
    if (session.value.admin) config.value = await api("/api/settings");
    editRules.value = {
      ...(n.rules ??
        config.value?.settings.rules ?? {
          cpu: 90,
          memory: 90,
          disk: 90,
          holdSeconds: 30,
          offlineSeconds: 15,
          intervalSeconds: 3,
        }),
    };
    await history();
  });
}
async function openSettings() {
  if (busy.value) return;
  tab.value = "settings";
  selected.value = "";
  await action(async () => {
    config.value = await api("/api/settings");
  });
}
async function add() {
  await action(async () => {
    credential.value = await api("/api/nodes", "POST", { name: name.value });
    name.value = "";
    await refresh();
  });
}
async function saveSettings() {
  await action(async () => {
    await api("/api/settings", "PUT", config.value!.settings);
    config.value = await api("/api/settings");
    notice.value = "設定已儲存";
  });
}
async function saveNode() {
  await action(async () => {
    await api(`/api/nodes/${selected.value}`, "PUT", {
      name: editName.value,
      rules: override.value ? editRules.value : null,
    });
    await refresh();
    notice.value = "主機設定已儲存";
  });
}
async function remove() {
  await action(async () => {
    await api(`/api/nodes/${selected.value}`, "DELETE");
    selected.value = "";
    await refresh();
  });
}
async function rotate() {
  await action(async () => {
    credential.value = await api(`/api/nodes/${selected.value}/token`, "POST");
    enroll.value = true;
  });
}
async function schedule(cancel = false) {
  await action(async () => {
    await api("/api/database", "PUT", { url: cancel ? "" : databaseURL.value });
    databaseURL.value = "";
    config.value = await api("/api/settings");
    notice.value = cancel ? "已取消資料庫切換" : "已排程，請重啟主程式進行搬移";
  });
}
async function copyToken() {
  await action(async () => {
    await navigator.clipboard.writeText(credential.value!.token);
    notice.value = "Token 已複製";
  });
}
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <a
        href="#"
        class="brand"
        @click.prevent="
          tab = 'overview';
          selected = '';
        "
        ><span class="brand-icon"><Activity :size="23" /></span
        ><strong>SPM<span class="brand-dot">.</span></strong></a
      >
      <div class="nav-caption">WORKSPACE</div>
      <nav aria-label="主導覽">
        <button
          :class="{ active: tab === 'overview' }"
          @click="
            tab = 'overview';
            selected = '';
          "
        >
          <LayoutDashboard :size="18" />監控總覽</button
        ><button
          :class="{ active: tab === 'alerts' }"
          @click="
            tab = 'alerts';
            selected = '';
          "
        >
          <Bell :size="18" />告警紀錄<span
            v-if="activeAlerts.length"
            class="nav-count"
            >{{ activeAlerts.length }}</span
          ></button
        ><button
          v-if="session.admin"
          :class="{ active: tab === 'settings' }"
          @click="openSettings"
        >
          <SettingsIcon :size="18" />系統設定
        </button>
      </nav>
      <div class="sidebar-bottom">
        <span class="status-dot" />{{
          session.public ? "公開監控面板" : "私人監控面板"
        }}<small>Simple. Precise. Monitored.</small>
      </div>
    </aside>
    <main>
      <header class="topbar">
        <div class="breadcrumb">
          工作空間 <span>/</span>
          {{
            tab === "settings"
              ? "系統設定"
              : tab === "alerts"
                ? "告警紀錄"
                : "監控總覽"
          }}
        </div>
        <div class="top-actions">
          <span class="live-pill"><span class="status-dot" />LIVE · 3s</span
          ><button
            v-if="session.admin"
            class="icon-button"
            aria-label="登出"
            @click="logout"
          >
            <LogOut :size="18" /></button
          ><button v-else class="small-button" @click="loginOpen = true">
            <Shield :size="15" />管理員登入
          </button>
        </div>
      </header>
      <div class="content">
        <div v-if="error" role="alert" class="banner error">{{ error }}</div>
        <div v-if="notice" role="status" class="banner success">
          {{ notice }}
        </div>
        <div v-if="!ready" class="empty-state">正在連線至監控服務…</div>
        <section
          v-else-if="!session.admin && !session.public"
          class="login-panel panel"
        >
          <span class="eyebrow">PRIVATE WORKSPACE</span>
          <h1>登入監控工作空間</h1>
          <p class="muted">此面板僅限管理員存取。</p>
          <form @submit.prevent="login">
            <label
              >帳號<input
                v-model="username"
                autocomplete="username"
                required /></label
            ><label
              >密碼<input
                v-model="password"
                type="password"
                autocomplete="current-password"
                required /></label
            ><button class="primary" :disabled="busy">登入</button>
          </form>
        </section>
        <template v-else>
          <template v-if="tab === 'overview' && !selected"
            ><div class="page-heading">
              <div>
                <span class="eyebrow">INFRASTRUCTURE OVERVIEW</span>
                <h1>掌握每一台主機<span class="accent">。</span></h1>
                <p class="muted">即時狀態、資源用量與連線健康，一目了然。</p>
              </div>
              <button
                v-if="session.admin"
                class="primary"
                @click="
                  credential = null;
                  enroll = true;
                "
              >
                <Plus :size="17" />新增主機
              </button>
            </div>
            <div class="stats">
              <article class="stat panel">
                <span>線上主機 <Server :size="17" /></span
                ><strong
                  >{{ online.length
                  }}<small>/ {{ nodes.length }}</small></strong
                >
                <p>
                  <span class="status-dot" />{{ nodes.length - online.length }}
                  台離線或等待連線
                </p>
              </article>
              <article class="stat panel">
                <span>平均 CPU <Activity :size="17" /></span
                ><strong>{{ percent(average) }}<small>%</small></strong>
                <p>所有線上主機的平均負載</p>
              </article>
              <article class="stat panel">
                <span>總接收速率 <ArrowDown :size="17" /></span
                ><strong class="rate-stat"
                  >{{ bytes(totalRX) }}<small>/s</small></strong
                >
                <p>線上主機網路流量</p>
              </article>
              <article class="stat panel">
                <span>近期未恢復告警 <Bell :size="17" /></span
                ><strong :class="{ 'warning-text': activeAlerts.length }"
                  >{{ activeAlerts.length }}<small>筆</small></strong
                >
                <p>依最近 100 筆事件計算</p>
              </article>
            </div>
            <div class="section-title host-toolbar">
              <h2>
                主機列表 <span class="count">{{ nodes.length }}</span>
              </h2>
              <label class="search"
                ><Search :size="17" /><input
                  v-model="search"
                  placeholder="搜尋主機名稱、系統…"
                  aria-label="搜尋主機"
              /></label>
            </div>
            <div v-if="!visible.length" class="empty-state panel">
              <Server :size="32" />
              <h3>
                {{ nodes.length ? "沒有符合的主機" : "開始監控你的第一台主機" }}
              </h3>
              <p>
                {{
                  nodes.length
                    ? "試試其他搜尋關鍵字。"
                    : "新增主機並啟動 Agent，即可看到即時資訊。"
                }}
              </p>
            </div>
            <div class="node-grid">
              <button
                v-for="n in visible"
                :key="n.id"
                class="node-card panel"
                :data-testid="'node-' + n.id"
                @click="openNode(n)"
              >
                <div class="node-top">
                  <span class="node-icon"><Server :size="21" /></span
                  ><span class="node-title"
                    ><strong>{{ n.name }}</strong
                    ><small
                      >{{ n.latest?.snapshot.platform || "等待 Agent 連線" }} ·
                      {{ n.latest?.snapshot.arch || "—" }}</small
                    ></span
                  ><ArrowUpRight :size="18" class="muted" />
                </div>
                <div class="node-status">
                  <span :class="['status-dot', { offline: !n.online }]" />{{
                    n.online
                      ? "運作正常"
                      : n.lastSeen
                        ? "主機離線"
                        : "等待連線"
                  }}<span>{{ n.latest?.snapshot.cores ?? "—" }} vCPU</span>
                </div>
                <div class="meters">
                  <div
                    v-for="[key, label] in [
                      ['cpu', 'CPU'],
                      ['memory', '記憶體'],
                      ['disk', '磁碟'],
                    ] as const"
                    :key="key"
                  >
                    <div>
                      <span>{{ label }}</span
                      ><strong
                        >{{ percent(n.latest?.metrics[key])
                        }}<small>%</small></strong
                      >
                    </div>
                    <div class="meter">
                      <i
                        :class="{ hot: (n.latest?.metrics[key] ?? 0) > 90 }"
                        :style="{ width: (n.latest?.metrics[key] ?? 0) + '%' }"
                      />
                    </div>
                  </div>
                </div>
                <div class="node-footer">
                  <span
                    ><ArrowDown :size="13" />{{
                      bytes(n.latest?.metrics.rx)
                    }}/s</span
                  ><span
                    ><ArrowUp :size="13" />{{
                      bytes(n.latest?.metrics.tx)
                    }}/s</span
                  >
                </div>
              </button>
            </div>
          </template>
          <template v-else-if="tab === 'overview' && current"
            ><button class="back-button" @click="selected = ''">
              <ArrowLeft :size="16" />返回總覽
            </button>
            <div class="page-heading">
              <div>
                <span class="eyebrow">HOST DETAILS</span>
                <h1>{{ current.name }}</h1>
                <p class="muted">
                  {{ current.latest?.snapshot.hostname }} ·
                  {{ current.latest?.snapshot.os }} ·
                  {{ current.latest?.snapshot.cores }} vCPU
                  <span :class="['status-dot', { offline: !current.online }]" />
                  {{ current.online ? "線上" : "離線" }}
                </p>
              </div>
              <label
                >歷史範圍<select
                  v-model.number="hours"
                  @change="action(history)"
                >
                  <option :value="1">最近 1 小時</option>
                  <option :value="6">最近 6 小時</option>
                  <option :value="24">最近 24 小時</option>
                  <option :value="168">最近 7 天</option>
                </select></label
              >
            </div>
            <div class="charts">
              <Trend
                :points="points"
                metric="disk"
                title="磁碟使用率"
                unit="%"
              />
              <Trend
                :points="points"
                metric="cpu"
                title="CPU 使用率"
                unit="%"
              /><Trend
                :points="points"
                metric="memory"
                title="記憶體使用率"
                unit="%"
              /><Trend
                :points="points"
                metric="rx"
                title="網路接收"
                unit="B/s"
              /><Trend
                :points="points"
                metric="tx"
                title="網路傳送"
                unit="B/s"
              />
            </div>
            <section class="panel padded">
              <h2>系統資訊</h2>
              <div class="detail-grid">
                <div>
                  記憶體<strong
                    >{{ bytes(current.latest?.snapshot.memoryUsed) }} /
                    {{ bytes(current.latest?.snapshot.memoryTotal) }}</strong
                  >
                </div>
                <div>
                  連續運行<strong
                    >{{
                      ((current.latest?.snapshot.uptime ?? 0) / 86400).toFixed(
                        1,
                      )
                    }}
                    天</strong
                  >
                </div>
                <div>
                  最後上報<strong>{{
                    current.lastSeen ? date(current.lastSeen) : "尚未上報"
                  }}</strong>
                </div>
                <div v-for="d in current.latest?.snapshot.disks" :key="d.path">
                  磁碟 {{ d.path
                  }}<strong>{{ bytes(d.used) }} / {{ bytes(d.total) }}</strong>
                </div>
              </div>
            </section>
            <form
              v-if="session.admin"
              class="panel padded"
              @submit.prevent="saveNode"
            >
              <h2>主機設定</h2>
              <label
                >顯示名稱<input
                  v-model="editName"
                  required
                  maxlength="100" /></label
              ><label class="check"
                ><input
                  v-model="override"
                  type="checkbox"
                />覆寫全域採樣與告警規則</label
              ><RuleFields v-if="override" :rules="editRules" />
              <p class="muted">
                離線判定至少為採樣間隔的 3 倍加 3 秒。新間隔於下次上報後生效。
              </p>
              <div class="button-row">
                <button class="primary" :disabled="busy">儲存主機設定</button
                ><button type="button" @click="rotate" :disabled="busy">
                  <RefreshCw :size="15" />重設 Agent Token</button
                ><button
                  type="button"
                  class="danger"
                  @click="deleteConfirm = true"
                >
                  刪除主機
                </button>
              </div>
              <div v-if="deleteConfirm" class="banner error">
                將刪除主機及其歷史資料。<button
                  type="button"
                  @click="remove"
                  :disabled="busy"
                >
                  確認刪除</button
                ><button type="button" @click="deleteConfirm = false">
                  取消
                </button>
              </div>
            </form></template
          >
          <template v-else-if="tab === 'alerts'"
            ><div class="page-heading">
              <div>
                <span class="eyebrow">EVENT LOG</span>
                <h1>告警紀錄</h1>
                <p class="muted">追蹤異常與恢復事件，顯示最近 100 筆。</p>
              </div>
            </div>
            <div class="panel events">
              <div v-if="!alerts.length" class="empty-state">
                <Bell :size="32" />
                <h3>目前沒有告警紀錄</h3>
                <p>異常觸發與恢復後，事件會顯示於此。</p>
              </div>
              <article v-for="a in alerts" :key="a.id" class="event">
                <span :class="['event-icon', { triggered: a.active }]"
                  ><Bell :size="17"
                /></span>
                <div>
                  <strong>{{ a.nodeName }} · {{ a.kind }}</strong>
                  <p>{{ a.active ? "超過門檻或連線中斷" : "已恢復正常" }}</p>
                </div>
                <time>{{ date(a.time) }}</time
                ><span :class="['badge', { triggered: a.active }]">{{
                  a.active ? "觸發" : "恢復"
                }}</span>
              </article>
            </div></template
          >
          <template v-else-if="tab === 'settings' && config && session.admin"
            ><div class="page-heading">
              <div>
                <span class="eyebrow">WORKSPACE SETTINGS</span>
                <h1>系統設定</h1>
                <p class="muted">調整可見性、採集規則與通知方式。</p>
              </div>
            </div>
            <form class="settings-layout" @submit.prevent="saveSettings">
              <section class="panel padded">
                <h2>一般設定</h2>
                <label class="check"
                  ><input
                    v-model="config.settings.public"
                    type="checkbox"
                  />允許公開查看監控面板</label
                ><label
                  >歷史保留天數<input
                    v-model.number="config.settings.retentionDays"
                    type="number"
                    min="1"
                    max="365"
                    required
                /></label>
                <h2>全域採樣與告警</h2>
                <RuleFields :rules="config.settings.rules" />
                <p class="muted">CPU 與記憶體須持續超標；磁碟超標立即通知。</p>
              </section>
              <section class="panel padded">
                <h2>告警通知</h2>
                <p class="muted">
                  所有事件固定記錄在 Web 面板，可額外啟用下列通知。
                </p>
                <label class="check"
                  ><input
                    v-model="config.settings.notifications.webhookEnabled"
                    type="checkbox"
                  />Webhook</label
                ><label
                  >Webhook URL<input
                    v-model="config.settings.notifications.webhookURL"
                    type="url"
                    placeholder="https://example.com/notify"
                    :required="
                      config.settings.notifications.webhookEnabled
                    " /></label
                ><label class="check"
                  ><input
                    v-model="config.settings.notifications.telegramEnabled"
                    type="checkbox"
                  />Telegram</label
                ><label
                  >Bot Token<input
                    v-model="config.settings.notifications.telegramToken"
                    type="password"
                    autocomplete="new-password"
                    :placeholder="
                      config.telegramTokenSet
                        ? '已設定，留空保持原值'
                        : '輸入 Bot Token'
                    " /></label
                ><label
                  >Chat ID<input
                    v-model="config.settings.notifications.telegramChat"
                    :required="config.settings.notifications.telegramEnabled"
                /></label>
              </section>
              <button class="primary" :disabled="busy">儲存設定</button>
            </form>
            <form
              class="panel padded database-form"
              @submit.prevent="schedule()"
            >
              <h2>資料庫</h2>
              <p class="muted">目前：{{ config.database.active }}</p>
              <p v-if="config.database.error" class="warning-text">
                {{ config.database.error }}
              </p>
              <p v-if="config.database.pending">
                待搬移：{{ config.database.pending }}
              </p>
              <p v-if="config.database.environment" class="muted">
                目前由 DATABASE_URL 環境變數管理。
              </p>
              <template v-else
                ><label
                  >完整資料庫 URL<input
                    v-model="databaseURL"
                    type="password"
                    autocomplete="new-password"
                    placeholder="postgresql://user:password@host:5432/spm"
                    required
                /></label>
                <p class="muted">
                  切換將在重啟後搬移主機、歷史、設定與告警。目標必須為空，來源資料會保留。
                </p>
                <div class="button-row">
                  <button :disabled="busy">排程資料庫切換</button
                  ><button
                    v-if="config.database.pending"
                    type="button"
                    @click="schedule(true)"
                    :disabled="busy"
                  >
                    取消切換
                  </button>
                </div></template
              >
            </form></template
          >
        </template>
        <footer class="footer">
          <span
            >SPM <span class="muted">/ Server Performance Monitor</span></span
          ><span class="muted">為每一份運算資源，保持關注。</span>
        </footer>
      </div>
    </main>
    <div
      v-if="loginOpen && (session.public || session.admin)"
      class="modal-backdrop"
    >
      <section
        class="modal panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="login-title"
      >
        <button
          class="modal-close icon-button"
          aria-label="關閉登入"
          @click="loginOpen = false"
        >
          <X :size="20" />
        </button>
        <h2 id="login-title">管理員登入</h2>
        <form @submit.prevent="login">
          <label
            >帳號<input
              v-model="username"
              autocomplete="username"
              required /></label
          ><label
            >密碼<input
              v-model="password"
              type="password"
              autocomplete="current-password"
              required
          /></label>
          <p v-if="error" role="alert" class="warning-text">{{ error }}</p>
          <button class="primary" :disabled="busy">登入</button>
        </form>
      </section>
    </div>
    <div v-if="enroll && session.admin" class="modal-backdrop">
      <section
        class="modal panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="enroll-title"
      >
        <button
          class="modal-close icon-button"
          aria-label="關閉新增主機"
          @click="
            enroll = false;
            credential = null;
          "
        >
          <X :size="20" /></button
        ><span class="eyebrow">CONNECT A HOST</span>
        <h2 id="enroll-title">
          {{ credential ? "Agent 連線資訊" : "新增主機" }}
        </h2>
        <template v-if="credential"
          ><p class="muted">
            Token 僅顯示一次，請妥善保存並設定於 Agent 環境變數。
          </p>
          <label>SPM_TOKEN<input :value="credential.token" readonly /></label
          ><button @click="copyToken"><Copy :size="15" />複製 Token</button>
          <p class="muted">
            設定 SPM_SERVER 為本面板網址，搭配 SPM_TOKEN 執行 spm-agent。
            主機 ID 由 Server 自動管理，不需填寫。詳細部署方式請參考 README。
          </p></template
        >
        <form v-else @submit.prevent="add">
          <label
            >主機名稱<input
              v-model="name"
              placeholder="例如：台北正式環境"
              maxlength="100"
              required /></label
          ><button class="primary" :disabled="busy">建立主機</button>
        </form>
        <p v-if="error" role="alert" class="warning-text">{{ error }}</p>
      </section>
    </div>
  </div>
</template>
