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

import React, { lazy, Suspense, useContext, useMemo, useState } from 'react';
import { Route, Routes, useLocation, useParams } from 'react-router-dom';
import Loading from './components/common/ui/Loading';
import SetupCheck from './components/layout/SetupCheck';
import { StatusContext } from './context/Status';
import {
  AdminRoute,
  AuthRedirect,
  PrivateRoute,
  RootRoute,
} from './helpers/auth';
import {
  headerModuleRequiresAuth,
  normalizeHeaderNavModules,
} from './helpers/navigationConfig';
import { getStoredJSON } from './helpers/siteStorage';

const Home = lazy(() => import('./pages/Home'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const About = lazy(() => import('./pages/About'));
const Rankings = lazy(() => import('./pages/Rankings'));
const UserAgreement = lazy(() => import('./pages/UserAgreement'));
const PrivacyPolicy = lazy(() => import('./pages/PrivacyPolicy'));
const Support = lazy(() => import('./pages/Support'));
const User = lazy(() => import('./pages/User'));
const Setting = lazy(() => import('./pages/Setting'));
const Channel = lazy(() => import('./pages/Channel'));
const Token = lazy(() => import('./pages/Token'));
const Redemption = lazy(() => import('./pages/Redemption'));
const TopUp = lazy(() => import('./pages/TopUp'));
const ViolationAudit = lazy(() => import('./pages/ViolationAudit'));
const Log = lazy(() => import('./pages/Log'));
const Chat = lazy(() => import('./pages/Chat'));
const Chat2Link = lazy(() => import('./pages/Chat2Link'));
const Midjourney = lazy(() => import('./pages/Midjourney'));
const Pricing = lazy(() => import('./pages/Pricing'));
const Task = lazy(() => import('./pages/Task'));
const ModelPage = lazy(() => import('./pages/Model'));
const ModelDeploymentPage = lazy(() => import('./pages/ModelDeployment'));
const Playground = lazy(() => import('./pages/Playground'));
const Subscription = lazy(() => import('./pages/Subscription'));
const Setup = lazy(() => import('./pages/Setup'));
const RegisterForm = lazy(() => import('./components/auth/RegisterForm'));
const LoginForm = lazy(() => import('./components/auth/LoginForm'));
const PasswordResetForm = lazy(
  () => import('./components/auth/PasswordResetForm'),
);
const PasswordResetConfirm = lazy(
  () => import('./components/auth/PasswordResetConfirm'),
);
const OAuth2Callback = lazy(() => import('./components/auth/OAuth2Callback'));
const PersonalSetting = lazy(
  () => import('./components/settings/PersonalSetting'),
);
const NotFound = lazy(() => import('./pages/NotFound'));
const Forbidden = lazy(() => import('./pages/Forbidden'));

const requirePrivate = (element) => <PrivateRoute>{element}</PrivateRoute>;
const requireAdmin = (element) => <AdminRoute>{element}</AdminRoute>;
const requireRoot = (element) => <RootRoute>{element}</RootRoute>;

function DynamicOAuth2Callback() {
  const { provider } = useParams();
  return <OAuth2Callback type={provider} />;
}

function App() {
  const location = useLocation();
  const [statusState] = useContext(StatusContext);
  const [cachedStatus] = useState(() => getStoredJSON('status', {}));
  const effectiveStatus = useMemo(
    () =>
      statusState?.status && Object.keys(statusState.status).length > 0
        ? statusState.status
        : cachedStatus,
    [statusState?.status, cachedStatus],
  );
  const headerModules = useMemo(
    () => normalizeHeaderNavModules(effectiveStatus?.HeaderNavModules),
    [effectiveStatus?.HeaderNavModules],
  );

  const pricingRequireAuth = useMemo(
    () => headerModuleRequiresAuth(headerModules, 'pricing'),
    [headerModules],
  );
  const rankingsRequireAuth = useMemo(
    () => headerModuleRequiresAuth(headerModules, 'rankings'),
    [headerModules],
  );

  return (
    <SetupCheck>
      <Suspense fallback={<Loading />} key={location.pathname}>
        <Routes>
          <Route path='/' element={<Home />} />
          <Route path='/setup' element={<Setup />} />
          <Route path='/forbidden' element={<Forbidden />} />

          <Route path='/console/models' element={requireAdmin(<ModelPage />)} />
          <Route
            path='/console/deployment'
            element={requireAdmin(<ModelDeploymentPage />)}
          />
          <Route
            path='/console/subscription'
            element={requireAdmin(<Subscription />)}
          />
          <Route path='/console/channel' element={requireAdmin(<Channel />)} />
          <Route
            path='/console/violation-audit'
            element={requireAdmin(<ViolationAudit />)}
          />
          <Route
            path='/console/redemption'
            element={requireAdmin(<Redemption />)}
          />
          <Route path='/console/user' element={requireAdmin(<User />)} />
          <Route path='/console/setting' element={requireRoot(<Setting />)} />

          <Route path='/console' element={requirePrivate(<Dashboard />)} />
          <Route path='/console/token' element={requirePrivate(<Token />)} />
          <Route
            path='/console/playground'
            element={requirePrivate(<Playground />)}
          />
          <Route
            path='/console/personal'
            element={requirePrivate(<PersonalSetting />)}
          />
          <Route path='/console/topup' element={requirePrivate(<TopUp />)} />
          <Route
            path='/console/support'
            element={requirePrivate(<Support />)}
          />
          <Route path='/console/log' element={requirePrivate(<Log />)} />
          <Route
            path='/console/midjourney'
            element={requirePrivate(<Midjourney />)}
          />
          <Route path='/console/task' element={requirePrivate(<Task />)} />

          <Route path='/console/chat/:id?' element={requirePrivate(<Chat />)} />
          <Route path='/chat2link' element={requirePrivate(<Chat2Link />)} />

          <Route
            path='/pricing'
            element={
              pricingRequireAuth ? requirePrivate(<Pricing />) : <Pricing />
            }
          />
          <Route
            path='/rankings'
            element={
              rankingsRequireAuth ? requirePrivate(<Rankings />) : <Rankings />
            }
          />
          <Route path='/about' element={<About />} />
          <Route path='/user-agreement' element={<UserAgreement />} />
          <Route path='/privacy-policy' element={<PrivacyPolicy />} />

          <Route path='/user/reset' element={<PasswordResetConfirm />} />
          <Route
            path='/login'
            element={
              <AuthRedirect>
                <LoginForm />
              </AuthRedirect>
            }
          />
          <Route
            path='/register'
            element={
              <AuthRedirect>
                <RegisterForm />
              </AuthRedirect>
            }
          />
          <Route path='/reset' element={<PasswordResetForm />} />
          <Route
            path='/oauth/github'
            element={<OAuth2Callback type='github' />}
          />
          <Route
            path='/oauth/discord'
            element={<OAuth2Callback type='discord' />}
          />
          <Route path='/oauth/oidc' element={<OAuth2Callback type='oidc' />} />
          <Route
            path='/oauth/linuxdo'
            element={<OAuth2Callback type='linuxdo' />}
          />
          <Route path='/oauth/:provider' element={<DynamicOAuth2Callback />} />

          <Route path='*' element={<NotFound />} />
        </Routes>
      </Suspense>
    </SetupCheck>
  );
}

export default App;
