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

const getStorage = () => {
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      return window.localStorage;
    }
    if (typeof localStorage !== 'undefined') return localStorage;
  } catch {
    // Storage may be disabled by the browser or blocked in a private frame.
  }
  return null;
};

export function getStoredValue(key, fallback = null) {
  try {
    return getStorage()?.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

export function setStoredValue(key, value) {
  try {
    const storage = getStorage();
    if (!storage) return false;
    storage.setItem(key, String(value));
    return true;
  } catch {
    return false;
  }
}

/**
 * Read a JSON value from browser storage without allowing malformed cache data
 * to break page initialization.
 */
export function getStoredJSON(key, fallback = null) {
  const rawValue = getStoredValue(key, null);
  if (rawValue === null || rawValue === '') return fallback;

  try {
    return JSON.parse(rawValue);
  } catch {
    return fallback;
  }
}

export function setStoredJSON(key, value) {
  try {
    return setStoredValue(key, JSON.stringify(value));
  } catch {
    return false;
  }
}

export function removeStoredValue(key) {
  try {
    getStorage()?.removeItem(key);
  } catch {
    // Ignore unavailable storage; callers can continue with in-memory state.
  }
}

export function getStoredUser() {
  const rawUser = getStoredValue('user', '');
  if (!rawUser) return null;

  try {
    const user = JSON.parse(rawUser);
    return user && typeof user === 'object' && !Array.isArray(user)
      ? user
      : null;
  } catch {
    return null;
  }
}

export function isAdmin() {
  return Number(getStoredUser()?.role) >= 10;
}

export function isRoot() {
  return Number(getStoredUser()?.role) >= 100;
}

export function getSystemName() {
  return getStoredValue('system_name', '') || 'New API';
}

export function getLogo() {
  return getStoredValue('logo', '') || '/logo.png';
}

export function getUserIdFromLocalStorage() {
  return getStoredUser()?.id ?? -1;
}

export function getUserGroupFromLocalStorage() {
  return getStoredUser()?.group;
}

export function getFooterHTML() {
  return getStoredValue('footer_html');
}
