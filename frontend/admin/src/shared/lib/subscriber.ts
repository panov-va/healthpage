import type { SubscriberChannel, SubscriberScope } from "@/shared/api";

const CHANNEL_LABELS: Record<string, string> = {
  email: "Email",
  telegram: "Telegram",
  max: "MAX",
  slack: "Slack",
  rss: "RSS",
  ical: "iCal",
  webhook: "Webhook",
};

export function channelLabel(c: SubscriberChannel): string {
  return CHANNEL_LABELS[c] ?? c;
}

export function scopeLabel(s: SubscriberScope): string {
  return s === "components" ? "Компоненты" : "Вся страница";
}
