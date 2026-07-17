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

import { Toast } from '@douyinfe/semi-ui';
import { removeStoredValue } from './siteStorage';

export const isRequestCancellation = (error) =>
  error?.name === 'AbortError' ||
  error?.name === 'CanceledError' ||
  error?.code === 'ERR_CANCELED' ||
  error?.__CANCEL__ === true;

export function showError(error) {
  if (error?.__globalErrorHandled) {
    return;
  }
  console.error(error);
  if (isRequestCancellation(error)) {
    return;
  }
  if (!error?.message) {
    Toast.error('错误：' + error);
    return;
  }

  if (error.name !== 'AxiosError') {
    Toast.error('错误：' + error.message);
    return;
  }

  switch (error.response?.status) {
    case 401:
      removeStoredValue('user');
      window.location.href = '/login?expired=true';
      break;
    case 429:
      Toast.error('错误：请求次数过多，请稍后再试！');
      break;
    case 500:
      Toast.error('错误：服务器内部错误，请联系管理员！');
      break;
    case 405:
      Toast.info('本站仅作演示之用，无服务端！');
      break;
    default:
      Toast.error('错误：' + error.message);
  }
}

export function showWarning(message) {
  Toast.warning(message);
}

export function showSuccess(message) {
  Toast.success(message);
}

export function showInfo(message) {
  Toast.info(message);
}

export function showNotice(message, isHTML = false) {
  if (isHTML) {
    import('./htmlNotice')
      .then(({ showHtmlNotice }) => showHtmlNotice(message))
      .catch(() => Toast.info(message));
    return;
  }
  Toast.info(message);
}
