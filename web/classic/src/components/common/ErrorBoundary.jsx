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
import { Button } from '@douyinfe/semi-ui';
import { withTranslation } from 'react-i18next';
import ErrorState from './ErrorState';

class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false, routeKey: props.routeKey };
  }

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  static getDerivedStateFromProps(props, state) {
    if (props.routeKey !== state.routeKey) {
      return { hasError: false, routeKey: props.routeKey };
    }
    return null;
  }

  componentDidCatch(error, errorInfo) {
    console.error('[ErrorBoundary]', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      const { t } = this.props;
      return (
        <ErrorState
          code='ERROR'
          description={t('页面渲染出错，请刷新页面重试')}
          action={
            <Button
              theme='solid'
              type='primary'
              onClick={() => window.location.reload()}
            >
              {t('刷新页面')}
            </Button>
          }
        />
      );
    }
    return this.props.children;
  }
}

export default withTranslation()(ErrorBoundary);
