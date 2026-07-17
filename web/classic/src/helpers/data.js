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

import { removeStoredValue, setStoredValue } from './siteStorage';

export function setStatusData(data) {
  setStoredValue('status', JSON.stringify(data));
  setStoredValue('system_name', data.system_name);
  setStoredValue('logo', data.logo);
  setStoredValue('footer_html', data.footer_html);
  setStoredValue('quota_per_unit', data.quota_per_unit);
  // 兼容：保留旧字段，同时写入新的额度展示类型
  setStoredValue('display_in_currency', data.display_in_currency);
  setStoredValue('quota_display_type', data.quota_display_type || 'USD');
  setStoredValue('enable_drawing', data.enable_drawing);
  setStoredValue('enable_task', data.enable_task);
  setStoredValue('enable_data_export', data.enable_data_export);
  setStoredValue('chats', JSON.stringify(data.chats));
  setStoredValue('data_export_default_time', data.data_export_default_time);
  setStoredValue('default_collapse_sidebar', data.default_collapse_sidebar);
  setStoredValue('mj_notify_enabled', data.mj_notify_enabled);
  if (data.chat_link) {
    // localStorage.setItem('chat_link', data.chat_link);
  } else {
    removeStoredValue('chat_link');
  }
  if (data.chat_link2) {
    // localStorage.setItem('chat_link2', data.chat_link2);
  } else {
    removeStoredValue('chat_link2');
  }
  if (data.docs_link) {
    setStoredValue('docs_link', data.docs_link);
  } else {
    removeStoredValue('docs_link');
  }
}

export function setUserData(data) {
  setStoredValue('user', JSON.stringify(data));
}
