"use client";

// Notification channels now live in the settings modal. This route renders
// the same shared <NotificationsPanel/> so old links keep working.

import { NotificationsPanel } from "@/components/settings/notifications-panel";

export default function NotificationsPage() {
  return (
    <div className="page-root">
      <NotificationsPanel />
    </div>
  );
}
