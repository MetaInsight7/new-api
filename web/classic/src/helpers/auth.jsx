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

import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { getStoredUser } from './siteStorage';

export const hasRoleAtLeast = (user, minimumRole) => {
  const role = Number(user?.role);
  return Number.isFinite(role) && role >= minimumRole;
};

export function authHeader() {
  // return authorization header with jwt token
  const user = getStoredUser();

  if (user && user.token) {
    return { Authorization: 'Bearer ' + user.token };
  } else {
    return {};
  }
}

export const AuthRedirect = ({ children }) => {
  const user = getStoredUser();

  if (user) {
    return <Navigate to='/console' replace />;
  }

  return children;
};

function PrivateRoute({ children }) {
  const location = useLocation();

  if (!getStoredUser()) {
    return <Navigate to='/login' replace state={{ from: location }} />;
  }
  return children;
}

export function AdminRoute({ children }) {
  const location = useLocation();
  const user = getStoredUser();
  if (!user) {
    return <Navigate to='/login' replace state={{ from: location }} />;
  }
  // The API may serialise role as either a number or a numeric string.
  if (hasRoleAtLeast(user, 10)) {
    return children;
  }
  return <Navigate to='/forbidden' replace />;
}

export function RootRoute({ children }) {
  const location = useLocation();
  const user = getStoredUser();

  if (!user) {
    return <Navigate to='/login' replace state={{ from: location }} />;
  }

  if (hasRoleAtLeast(user, 100)) {
    return children;
  }

  return <Navigate to='/forbidden' replace />;
}

export { PrivateRoute };
