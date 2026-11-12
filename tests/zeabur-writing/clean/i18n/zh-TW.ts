import type { Messages } from "./en"

// 註解不檢查,進行
const latinEnd = (text: string) => (/[A-Za-z0-9]$/.test(text) ? " " : "")

export const zhTW: Messages = {
  "service.count": (n: number) => `${n} 個服務`,
  "refused.rightsFor": (what: string) => `${what}${latinEnd(what)}需要更多權限。`,
  "status.saving": "正在儲存…",
  "cluster.sourceOther": "—",
}
