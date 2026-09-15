export interface Rules {
  cpu: number;
  memory: number;
  disk: number;
  holdSeconds: number;
  offlineSeconds: number;
  intervalSeconds: number;
}
export interface Metrics {
  cpu: number | null;
  memory: number;
  disk: number;
  rx: number | null;
  tx: number | null;
}
export interface Point extends Metrics {
  time: number;
}
export interface Node {
  id: string;
  name: string;
  online: boolean;
  lastSeen: number;
  rules: Rules | null;
  latest: null | {
    time: number;
    metrics: Metrics;
    snapshot: {
      hostname: string;
      os: string;
      platform: string;
      arch: string;
      cores: number;
      uptime: number;
      memoryTotal: number;
      memoryUsed: number;
      disks: { path: string; total: number; used: number }[];
    };
  };
}
export interface Alert {
  id: string;
  nodeId: string;
  nodeName: string;
  kind: string;
  active: boolean;
  time: number;
  message: string;
}
export interface Settings {
  public: boolean;
  rules: Rules;
  retentionDays: number;
  notifications: {
    webhookEnabled: boolean;
    webhookURL: string;
    telegramEnabled: boolean;
    telegramToken: string;
    telegramChat: string;
  };
}
export interface Config {
  settings: Settings;
  telegramTokenSet: boolean;
  database: {
    active: string;
    pending: string;
    environment: boolean;
    error: string;
  };
}
