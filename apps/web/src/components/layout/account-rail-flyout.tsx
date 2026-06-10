"use client";

/**
 * AccountRailFlyout —— hover 弹出的二级面板。
 *
 * 当鼠标停留在 AccountRail（左侧 64px 极窄栏）上 0.3s 时显示，
 * 鼠标离开二级栏区域后 0.2s 自动收起。
 *
 * 内容：默认跟随轻松/完整模式过滤功能面，避免新用户一开始看到全部条目。
 * 通过 `data-active` 在当前路径上高亮。
 */

import { useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { NAV_ITEMS, NAV_GROUP_ORDER, type NavGroup, type NavItem, filterNavItemsByProfile, groupNavItems, navItemLabel } from "@/lib/nav-items";
import { useI18n } from "@/lib/i18n";
import { PROFILE_MODE_KEY, readProfileMode } from "@/lib/profile-mode";

interface AccountRailFlyoutProps {
  open: boolean;
  extItems?: NavItem[];
  /** 浮卡顶部相对视口的 px 位置（由父组件根据触发器 boundingRect 计算）。 */
  anchorTop?: number;
  onMouseEnter?: () => void;
  onMouseLeave?: () => void;
  onPick?: () => void;
}

const GROUP_LABEL_EN: Record<NavGroup, string> = {
  概览: "Overview",
  工作: "Work",
  智能: "Intelligence",
  系统: "System",
  扩展: "Extensions",
};

export default function AccountRailFlyout({ open, extItems = [], anchorTop = 0, onMouseEnter, onMouseLeave, onPick }: AccountRailFlyoutProps) {
  const router = useRouter();
  const pathname = usePathname();
  const { locale, t } = useI18n();
  const [profileMode, setProfileMode] = useState(readProfileMode);

  useEffect(() => {
    const handleProfileModeChange = (event: StorageEvent) => {
      if (event.key === PROFILE_MODE_KEY) setProfileMode(readProfileMode());
    };
    window.addEventListener("storage", handleProfileModeChange);
    return () => window.removeEventListener("storage", handleProfileModeChange);
  }, []);

  const grouped = useMemo(
    () => groupNavItems(filterNavItemsByProfile([...NAV_ITEMS, ...extItems], profileMode)),
    [extItems, profileMode],
  );
  const isZh = locale === "zh";
  const groupLabel = (g: NavGroup) => (isZh ? g : GROUP_LABEL_EN[g]);
  const title = isZh
    ? (profileMode === "easy" ? "主路径" : "所有功能")
    : (profileMode === "easy" ? "Main Path" : "All Features");

  return (
    <aside
      className="account-rail-flyout"
      data-open={open || undefined}
      style={{ top: anchorTop }}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      aria-hidden={!open}
      inert={!open}
    >
      <div className="account-rail-flyout-header">
        <div className="account-rail-flyout-title">
          {title}
        </div>
        <div className="account-rail-flyout-hint">
          {isZh ? (
            <>按 <kbd className="cmd-kbd-sm">⌘K</kbd> 快速搜索</>
          ) : (
            <>Press <kbd className="cmd-kbd-sm">⌘K</kbd> to search</>
          )}
        </div>
      </div>
      <div className="account-rail-flyout-body">
        {NAV_GROUP_ORDER.map((g) => {
          const items = grouped[g];
          if (!items || items.length === 0) return null;
          return (
            <section key={g} className="account-rail-flyout-group">
              <h3 className="account-rail-flyout-group-title">{groupLabel(g)}</h3>
              <div className="account-rail-flyout-list">
                {items.map((it) => {
                  const active = pathname === it.href || (pathname?.startsWith(it.href + "/") ?? false);
                  return (
                    <button
                      key={it.id}
                      type="button"
                      className="account-rail-flyout-item"
                      data-active={active || undefined}
                      onClick={() => {
                        router.push(it.href);
                        onPick?.();
                      }}
                    >
                      <span className="account-rail-flyout-item-icon">{it.icon}</span>
                      <span className="account-rail-flyout-item-label">{navItemLabel(it, t)}</span>
                    </button>
                  );
                })}
              </div>
            </section>
          );
        })}
      </div>
    </aside>
  );
}
