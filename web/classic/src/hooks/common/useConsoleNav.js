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

import { useEffect, useMemo, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  LayoutDashboard,
  MessageSquare,
  Server,
  User,
  ShieldCheck,
} from 'lucide-react';
import { useSidebar } from './useSidebar';
import { isAdmin, isRoot, showError } from '../../helpers';

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
  const location = useLocation();
  const [chatItems, setChatItems] = useState([]);

  const enableDataExport =
    localStorage.getItem('enable_data_export') === 'true';
  const enableDrawing = localStorage.getItem('enable_drawing') === 'true';
  const enableTask = localStorage.getItem('enable_task') === 'true';

  // 加载聊天项（与历史逻辑一致）
  useEffect(() => {
    let chats = localStorage.getItem('chats');
    if (!chats) return;
    try {
      chats = JSON.parse(chats);
      if (!Array.isArray(chats)) return;
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

    // 3. 控制台
    const consoleChildren = [];
    if (isModuleVisible('console', 'token')) {
      consoleChildren.push({
        key: 'token',
        label: t('令牌管理'),
        to: consoleRouterMap.token,
      });
    }
    if (isModuleVisible('console', 'log')) {
      consoleChildren.push({
        key: 'log',
        label: t('使用日志'),
        to: consoleRouterMap.log,
      });
    }
    if (isModuleVisible('console', 'midjourney') && enableDrawing) {
      consoleChildren.push({
        key: 'midjourney',
        label: t('绘图日志'),
        to: consoleRouterMap.midjourney,
      });
    }
    if (isModuleVisible('console', 'task') && enableTask) {
      consoleChildren.push({
        key: 'task',
        label: t('任务日志'),
        to: consoleRouterMap.task,
      });
    }
    if (consoleChildren.length) {
      list.push({
        key: 'console',
        label: t('控制台'),
        icon: Server,
        children: consoleChildren,
      });
    }

    // 4. 个人中心
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

    // 5. 管理（仅管理员）
    if (isAdmin()) {
      const adminChildren = [];
      if (isModuleVisible('admin', 'channel')) {
        adminChildren.push({
          key: 'channel',
          label: t('渠道管理'),
          to: consoleRouterMap.channel,
        });
      }
      if (isModuleVisible('admin', 'subscription')) {
        adminChildren.push({
          key: 'subscription',
          label: t('订阅管理'),
          to: consoleRouterMap.subscription,
        });
      }
      if (isModuleVisible('admin', 'models')) {
        adminChildren.push({
          key: 'models',
          label: t('模型管理'),
          to: consoleRouterMap.models,
        });
      }
      if (isModuleVisible('admin', 'deployment')) {
        adminChildren.push({
          key: 'deployment',
          label: t('模型部署'),
          to: consoleRouterMap.deployment,
        });
      }
      if (isModuleVisible('admin', 'redemption')) {
        adminChildren.push({
          key: 'redemption',
          label: t('兑换码管理'),
          to: consoleRouterMap.redemption,
        });
      }
      if (isModuleVisible('admin', 'user')) {
        adminChildren.push({
          key: 'user',
          label: t('用户管理'),
          to: consoleRouterMap.user,
        });
      }
      if (isRoot() && isModuleVisible('admin', 'setting')) {
        adminChildren.push({
          key: 'setting',
          label: t('系统设置'),
          to: consoleRouterMap.setting,
        });
      }
      if (adminChildren.length) {
        list.push({
          key: 'admin',
          label: t('管理'),
          icon: ShieldCheck,
          children: adminChildren,
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
    cat.to || (cat.children && cat.children[0] && cat.children[0].to) || '/console';

  return { categories, selectedKey, activeCategory, categoryTarget, t };
};
