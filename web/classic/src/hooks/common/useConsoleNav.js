/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { useContext, useEffect, useMemo, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  LayoutDashboard,
  MessageSquare,
  KeyRound,
  ScrollText,
  Boxes,
  User,
  ShieldCheck,
  Headphones,
} from 'lucide-react';
import { useSidebar } from './useSidebar';
import { getStoredJSON, getStoredValue } from '../../helpers/siteStorage';
import { showError } from '../../helpers/notifications';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';

// 路由映射（与历史保持一致）
export const consoleRouterMap = {
  detail: '/console',
  token: '/console/token',
  log: '/console/log',
  midjourney: '/console/midjourney',
  task: '/console/task',
  playground: '/console/playground',
  topup: '/console/topup',
  personal: '/console/personal',
  channel: '/console/channel',
  subscription: '/console/subscription',
  models: '/console/models',
  deployment: '/console/deployment',
  redemption: '/console/redemption',
  user: '/console/user',
  setting: '/console/setting',
  violationAudit: '/console/violation-audit',
  support: '/console/support',
};

/**
 * 控制台导航树（DMIT 风格）：
 * 蓝栏 = 5 个大组（看板 / 聊天 / 控制台 / 个人中心 / 管理）
 * 二级 = 各组的子页，由 ConsoleSubNav 在内容区以 pill 形式渲染
 * SiderBar 与 ConsoleSubNav 共用此 hook，保证可见性/权限逻辑一致。
 */
export const useConsoleNav = () => {
  const { t } = useTranslation();
  const { isModuleVisible } = useSidebar();
  const [userState] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const location = useLocation();
  const [chatItems, setChatItems] = useState([]);

  const cachedStatus = getStoredJSON('status', {});
  const status =
    statusState?.status && Object.keys(statusState.status).length > 0
      ? statusState.status
      : cachedStatus;
  const asBoolean = (value) => value === true || value === 'true';
  const enableDataExport = asBoolean(
    status?.enable_data_export ?? getStoredValue('enable_data_export', ''),
  );
  const enableDrawing = asBoolean(
    status?.enable_drawing ?? getStoredValue('enable_drawing', ''),
  );
  const enableTask = asBoolean(
    status?.enable_task ?? getStoredValue('enable_task', ''),
  );
  const violationAuditEnabled = asBoolean(status?.violation_audit_enabled);

  // 加载聊天项（与历史逻辑一致）
  useEffect(() => {
    const chats = getStoredJSON('chats', []);
    if (!Array.isArray(chats)) return;
    try {
      const items = [];
      for (let i = 0; i < chats.length; i++) {
        let shouldSkip = false;
        let chat = {};
        for (let key in chats[i]) {
          let link = chats[i][key];
          if (typeof link !== 'string') continue;
          if (
            link.startsWith('fluent') ||
            link.startsWith('ccswitch') ||
            link.startsWith('deepchat')
          ) {
            shouldSkip = true;
            break;
          }
          chat.text = key;
          chat.itemKey = 'chat' + i;
          chat.to = '/console/chat/' + i;
        }
        if (shouldSkip || !chat.text) continue;
        items.push(chat);
      }
      setChatItems(items);
    } catch (e) {
      showError('聊天数据解析失败');
    }
  }, []);

  const categories = useMemo(() => {
    const list = [];

    // 1. 看板（直达，无子页）
    if (isModuleVisible('console', 'detail') && enableDataExport) {
      list.push({
        key: 'detail',
        label: t('数据看板'),
        icon: LayoutDashboard,
        to: consoleRouterMap.detail,
      });
    }

    // 2. 聊天
    const chatChildren = [];
    if (isModuleVisible('chat', 'playground')) {
      chatChildren.push({
        key: 'playground',
        label: t('操练场'),
        to: consoleRouterMap.playground,
      });
    }
    if (isModuleVisible('chat', 'chat')) {
      chatItems.forEach((ci) =>
        chatChildren.push({ key: ci.itemKey, label: ci.text, to: ci.to }),
      );
    }
    if (chatChildren.length) {
      list.push({
        key: 'chat',
        label: t('聊天'),
        icon: MessageSquare,
        children: chatChildren,
      });
    }

    // 3. 令牌（直达，独立突出——API 网关最高频页）
    if (isModuleVisible('console', 'token')) {
      list.push({
        key: 'token',
        label: t('令牌'),
        icon: KeyRound,
        to: consoleRouterMap.token,
      });
    }

    // 4. 日志（纯观测：使用 / 绘图 / 任务）
    const logChildren = [];
    if (isModuleVisible('console', 'log')) {
      logChildren.push({
        key: 'log',
        label: t('使用日志'),
        to: consoleRouterMap.log,
      });
    }
    if (isModuleVisible('console', 'midjourney') && enableDrawing) {
      logChildren.push({
        key: 'midjourney',
        label: t('绘图日志'),
        to: consoleRouterMap.midjourney,
      });
    }
    if (isModuleVisible('console', 'task') && enableTask) {
      logChildren.push({
        key: 'task',
        label: t('任务日志'),
        to: consoleRouterMap.task,
      });
    }
    if (logChildren.length) {
      list.push({
        key: 'log',
        label: t('日志'),
        icon: ScrollText,
        children: logChildren,
      });
    }

    // 5. 个人中心（钱包 + 个人设置）
    const personalChildren = [];
    if (isModuleVisible('personal', 'topup')) {
      personalChildren.push({
        key: 'topup',
        label: t('钱包管理'),
        to: consoleRouterMap.topup,
      });
    }
    if (isModuleVisible('personal', 'personal')) {
      personalChildren.push({
        key: 'personal',
        label: t('个人设置'),
        to: consoleRouterMap.personal,
      });
    }
    if (personalChildren.length) {
      list.push({
        key: 'personal',
        label: t('个人中心'),
        icon: User,
        children: personalChildren,
      });
    }

    // 6. 技术支持（所有已登录用户可用）
    list.push({
      key: 'support',
      label: t('技术支持'),
      icon: Headphones,
      to: consoleRouterMap.support,
    });

    // 7. 运营（仅管理员）：渠道 / 模型 / 模型部署 / 订阅 / 兑换码
    // 8. 系统（仅管理员）：用户 / 系统设置(仅 root)
    // 两组均读同一个 admin 配置区，仅展示层拆分以理清层次。
    if (Number(userState?.user?.role) >= 10) {
      const operationChildren = [];
      if (isModuleVisible('admin', 'channel')) {
        operationChildren.push({
          key: 'channel',
          label: t('渠道管理'),
          to: consoleRouterMap.channel,
        });
      }
      if (isModuleVisible('admin', 'models')) {
        operationChildren.push({
          key: 'models',
          label: t('模型管理'),
          to: consoleRouterMap.models,
        });
      }
      if (isModuleVisible('admin', 'deployment')) {
        operationChildren.push({
          key: 'deployment',
          label: t('模型部署'),
          to: consoleRouterMap.deployment,
        });
      }
      if (isModuleVisible('admin', 'subscription')) {
        operationChildren.push({
          key: 'subscription',
          label: t('订阅管理'),
          to: consoleRouterMap.subscription,
        });
      }
      if (isModuleVisible('admin', 'redemption')) {
        operationChildren.push({
          key: 'redemption',
          label: t('兑换码管理'),
          to: consoleRouterMap.redemption,
        });
      }
      if (operationChildren.length) {
        list.push({
          key: 'operation',
          label: t('运营'),
          icon: Boxes,
          children: operationChildren,
        });
      }

      const systemChildren = [];
      if (isModuleVisible('admin', 'user')) {
        systemChildren.push({
          key: 'user',
          label: t('用户管理'),
          to: consoleRouterMap.user,
        });
      }
      if (
        Number(userState?.user?.role) >= 100 &&
        isModuleVisible('admin', 'setting')
      ) {
        systemChildren.push({
          key: 'setting',
          label: t('系统设置'),
          to: consoleRouterMap.setting,
        });
      }
      if (violationAuditEnabled) {
        systemChildren.push({
          key: 'violationAudit',
          label: t('违规审计'),
          to: consoleRouterMap.violationAudit,
        });
      }
      if (systemChildren.length) {
        list.push({
          key: 'system',
          label: t('系统'),
          icon: ShieldCheck,
          children: systemChildren,
        });
      }
    }

    return list;
  }, [
    t,
    chatItems,
    isModuleVisible,
    enableDataExport,
    enableDrawing,
    enableTask,
    violationAuditEnabled,
    status?.enable_data_export,
    status?.enable_drawing,
    status?.enable_task,
    status?.violation_audit_enabled,
    userState?.user?.role,
  ]);

  // 当前选中的子项 key（用于高亮）
  const selectedKey = useMemo(() => {
    const path = location.pathname;
    let key = Object.keys(consoleRouterMap).find(
      (k) => consoleRouterMap[k] === path,
    );
    if (!key && path.startsWith('/console/chat/')) {
      const idx = path.split('/').pop();
      key = !isNaN(idx) ? 'chat' + idx : 'chat';
    }
    return key;
  }, [location.pathname]);

  // 当前所属大组
  const activeCategory = useMemo(
    () =>
      categories.find(
        (c) =>
          c.key === selectedKey ||
          (c.children || []).some((ch) => ch.key === selectedKey),
      ) || null,
    [categories, selectedKey],
  );

  // 大组的导航目标（直达页 或 第一个子页）
  const categoryTarget = (cat) =>
    cat.to ||
    (cat.children && cat.children[0] && cat.children[0].to) ||
    '/console';

  return { categories, selectedKey, activeCategory, categoryTarget, t };
};
