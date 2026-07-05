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

import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { API, copy, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useActualTheme } from '../../context/Theme';
import { StatusContext } from '../../context/Status';
import NoticeModal from '../../components/layout/NoticeModal';
import './home.css';

const METRICS = [
  {
    accent: 'coral',
    stamp: 'MODEL',
    label: '🎒 朋友多',
    value: '200+ 主流模型',
    copy: '一个接口，接入全网主流大模型。',
  },
  {
    accent: 'green',
    stamp: 'SLA',
    label: '🛡️ 下盘稳',
    value: '99.9% 高可用保障',
    copy: '多节点冗余，故障自动切换。',
  },
  {
    accent: 'blue',
    stamp: '/V1',
    label: '🔌 不挑食',
    value: '多协议兼容',
    copy: '适配 Claude、Codex、Cursor 等 AI 工具。',
  },
  {
    accent: 'yellow',
    stamp: '30S',
    label: '⏱️ 手脚快',
    value: '30s 极速接入',
    copy: '换条地址，灵感不用等。',
  },
];

const STEPS = [
  ['01', '领取通行证', '注册获取你的专属 API Key。'],
  ['02', '换上地址', '填入一行专属 Base URL。'],
  ['03', '叫上搭子，开工！', '全球主流模型，随时响应你的指令。'],
];

const API_BASE_PATH = '/v1';

const escapeHtml = (value) =>
  value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');

const getCachedServerAddress = () => {
  if (typeof window === 'undefined') return '';

  try {
    const status = JSON.parse(localStorage.getItem('status') || '{}');
    return typeof status.server_address === 'string'
      ? status.server_address.trim()
      : '';
  } catch (error) {
    console.error('读取服务器地址缓存失败:', error);
    return '';
  }
};

const getServerAddress = (status) => {
  const serverAddress =
    typeof status?.server_address === 'string'
      ? status.server_address.trim()
      : '';

  if (serverAddress) return serverAddress;

  const cachedServerAddress = getCachedServerAddress();
  if (cachedServerAddress) return cachedServerAddress;

  return typeof window === 'undefined' ? '' : window.location.origin;
};

const getApiBaseURL = (serverAddress) => {
  const normalized = serverAddress.trim().replace(/\/+$/, '');
  if (!normalized) return API_BASE_PATH;
  if (normalized.endsWith(API_BASE_PATH)) return normalized;
  return `${normalized}${API_BASE_PATH}`;
};

const getApiBaseHost = (apiBaseURL) => {
  try {
    const url = new URL(apiBaseURL);
    return `${url.host}${url.pathname}`.replace(/\/+$/, '');
  } catch (error) {
    return apiBaseURL.replace(/^https?:\/\//, '').replace(/\/+$/, '');
  }
};

const createCodeSamples = (apiBaseURL) => {
  const safeApiBaseURL = escapeHtml(apiBaseURL);

  return {
    python: {
      title: 'quick-start.py',
      code: `<span class="c-kw">from</span> openai <span class="c-kw">import</span> <span class="c-ty">OpenAI</span>

client = <span class="c-ty">OpenAI</span>(
    api_key=<span class="c-str">"tdz_xxx"</span>,
    base_url=<span class="hot"><span class="c-str">"${safeApiBaseURL}"</span></span>,  <span class="hot-cm"># 改这行就够了</span>
)

resp = client.chat.completions.<span class="c-fn">create</span>(
    model=<span class="c-str">"claude-sonnet"</span>,
    messages=[{<span class="c-str">"role"</span>: <span class="c-str">"user"</span>, <span class="c-str">"content"</span>: <span class="c-str">"写个快排"</span>}],
)
<span class="c-fn">print</span>(resp.choices[<span class="c-num">0</span>].message.content)`,
    },
    curl: {
      title: 'request.sh',
      code: `<span class="c-cm"># 换条地址，原样调用</span>
curl <span class="hot">${safeApiBaseURL}</span>/chat/completions \\
  -H <span class="c-str">"Authorization: Bearer tdz_xxx"</span> \\
  -H <span class="c-str">"Content-Type: application/json"</span> \\
  -d <span class="c-str">'{
    "model": "claude-sonnet",
    "messages": [{"role": "user", "content": "写个快排"}]
  }'</span>`,
    },
    js: {
      title: 'quick-start.ts',
      code: `<span class="c-kw">import</span> <span class="c-ty">OpenAI</span> <span class="c-kw">from</span> <span class="c-str">"openai"</span>;

<span class="c-kw">const</span> client = <span class="c-kw">new</span> <span class="c-ty">OpenAI</span>({
  apiKey: <span class="c-str">"tdz_xxx"</span>,
  baseURL: <span class="hot"><span class="c-str">"${safeApiBaseURL}"</span></span>,  <span class="hot-cm">// 改这行就够了</span>
});

<span class="c-kw">const</span> resp = <span class="c-kw">await</span> client.chat.completions.<span class="c-fn">create</span>({
  model: <span class="c-str">"claude-sonnet"</span>,
  messages: [{ role: <span class="c-str">"user"</span>, content: <span class="c-str">"写个快排"</span> }],
});
console.<span class="c-fn">log</span>(resp.choices[<span class="c-num">0</span>].message.content);`,
    },
    go: {
      title: 'main.go',
      code: `client := openai.<span class="c-fn">NewClient</span>(
  option.<span class="c-fn">WithAPIKey</span>(<span class="c-str">"tdz_xxx"</span>),
  option.<span class="c-fn">WithBaseURL</span>(<span class="hot"><span class="c-str">"${safeApiBaseURL}"</span></span>),  <span class="hot-cm">// 改这行就够了</span>
)

resp, _ := client.Chat.Completions.<span class="c-fn">New</span>(ctx, openai.<span class="c-ty">ChatCompletionNewParams</span>{
  Model:    <span class="c-str">"claude-sonnet"</span>,
  Messages: []openai.<span class="c-ty">ChatCompletionMessageParamUnion</span>{
    openai.<span class="c-fn">UserMessage</span>(<span class="c-str">"写个快排"</span>),
  },
})`,
    },
    rust: {
      title: 'main.rs',
      code: `<span class="c-kw">let</span> client = <span class="c-ty">Client</span>::<span class="c-fn">new</span>();

<span class="c-kw">let</span> resp = client
    .<span class="c-fn">post</span>(<span class="hot"><span class="c-str">"${safeApiBaseURL}/chat/completions"</span></span>)  <span class="hot-cm">// 改这行就够了</span>
    .<span class="c-fn">bearer_auth</span>(<span class="c-str">"tdz_xxx"</span>)
    .<span class="c-fn">json</span>(&<span class="c-fn">json!</span>({
        <span class="c-str">"model"</span>: <span class="c-str">"claude-sonnet"</span>,
        <span class="c-str">"messages"</span>: [{<span class="c-str">"role"</span>: <span class="c-str">"user"</span>, <span class="c-str">"content"</span>: <span class="c-str">"写个快排"</span>}]
    }))
    .<span class="c-fn">send</span>()
    .<span class="c-kw">await</span>?;`,
    },
  };
};

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(true);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [activeLanguage, setActiveLanguage] = useState('python');
  const [copiedButton, setCopiedButton] = useState('');
  const isMobile = useIsMobile();
  const serverAddress = getServerAddress(statusState?.status);
  const apiBaseURL = getApiBaseURL(serverAddress);
  const apiBaseHost = getApiBaseHost(apiBaseURL);
  const codeSamples = useMemo(
    () => createCodeSamples(apiBaseURL),
    [apiBaseURL],
  );
  const activeSample = codeSamples[activeLanguage];

  const displayHomePageContent = async () => {
    try {
      const res = await API.get('/api/home_page_content', {
        skipErrorHandler: true,
        timeout: 5000,
      });
      const { success, data } = res.data;
      if (success && typeof data === 'string') {
        let content = data;
        if (data.trim() === '') {
          setHomePageContent('');
          return;
        }
        if (!data.startsWith('https://')) {
          content = marked.parse(data);
        }
        setHomePageContent(content);

        if (data.startsWith('https://')) {
          const iframe = document.querySelector('iframe');
          if (iframe) {
            iframe.onload = () => {
              iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
              iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
            };
          }
        }
      } else {
        setHomePageContent('');
      }
    } catch (error) {
      console.error('加载首页内容失败:', error);
      setHomePageContent('');
    }
    setHomePageContentLoaded(true);
  };

  const handleCopyBaseURL = (buttonKey = 'base') => {
    setCopiedButton(buttonKey);
    copy(apiBaseURL).catch((error) => {
      console.error('复制 Base URL 失败:', error);
    });
    showSuccess(t('已复制到剪切板'));
    window.setTimeout(() => setCopiedButton(''), 1200);
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  return (
    <div className='classic-page-fill classic-home-page token-dazi-home w-full overflow-x-hidden'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <div className='token-home'>
          <a className='token-skip-link' href='#main'>
            跳到主要内容
          </a>
          <div className='token-route-ribbon' aria-hidden='true'>
            <span>
              🐾 零学习成本 · 🛡️ 毫秒级线路容灾 · 🥟 随叫随到的搭子 · 🚀 99.9%
              节点高可用 · 🎨 Vibe Coding 绝配 · ⚡ 高并发智能选路 · 🐾
              零学习成本 · 🛡️ 毫秒级线路容灾 · 🥟 随叫随到的搭子 · 🚀 99.9%
              节点高可用 · 🎨 Vibe Coding 绝配 · ⚡ 高并发智能选路 ·
            </span>
          </div>

          <main id='main'>
            <section className='token-hero'>
              <div className='token-shell token-hero-layout'>
                <div className='token-hero-copy'>
                  <div className='token-kicker'>
                    <span />
                    OpenAI Compatible · /v1 Ready
                  </div>
                  <h1>
                    接上 Token 搭子，<em>大模型马上开工。</em>
                  </h1>
                  <p className='token-lead'>
                    <span>
                      一个接口，打包接入 GPT、Claude、Gemini、DeepSeek 全家桶。
                    </span>
                    稳定中转，按量计费，比官方更省心。
                  </p>
                  <div className='token-actions'>
                    <Link
                      className='token-btn token-btn-primary'
                      to='/register'
                    >
                      开启搭子之旅 <i aria-hidden='true'>›</i>
                    </Link>
                    <Link className='token-btn' to='/pricing'>
                      查看可用线路
                    </Link>
                  </div>
                  <div className='token-base-strip' aria-label='Base URL'>
                    <b>Base URL</b>
                    <code translate='no'>{apiBaseURL}</code>
                    <button
                      type='button'
                      onClick={() => handleCopyBaseURL('desktop')}
                      aria-label='复制 Base URL'
                    >
                      {copiedButton === 'desktop' ? '已复制' : '复制'}
                    </button>
                  </div>
                </div>

                <div className='token-passport' aria-label='模型通行证示意'>
                  <div className='token-pass-head'>
                    <div>
                      <strong>大模型通行证</strong>
                      <span translate='no'>Token Dazi · /v1 Access</span>
                    </div>
                    <div className='token-pass-badge'>线路畅通</div>
                  </div>
                  <div className='token-pass-body'>
                    <div className='token-route-ticket'>
                      <div className='token-route-title'>请求通行路线</div>
                      <div className='token-route-flow'>
                        <div className='token-route-item'>
                          <span
                            className='token-route-dot'
                            style={{ '--color': '#141418' }}
                          />
                          <div>
                            <b>你的应用 / IDE</b>
                            <span>Claude、Codex、脚本、服务端应用</span>
                          </div>
                          <code>client</code>
                        </div>
                        <div className='token-route-item'>
                          <span
                            className='token-route-dot'
                            style={{ '--color': '#15a77a' }}
                          />
                          <div>
                            <b>Token 搭子通行口</b>
                            <span>统一鉴权、线路、用量记录</span>
                          </div>
                          <code>/v1</code>
                        </div>
                        <div className='token-route-item'>
                          <span
                            className='token-route-dot'
                            style={{ '--color': '#326ef1' }}
                          />
                          <div>
                            <b>Claude / DeepSeek / Qwen</b>
                            <span>需要哪位搭子，就叫哪位</span>
                          </div>
                          <code>ready</code>
                        </div>
                      </div>
                      <div className='token-stamp'>
                        OpenAI
                        <br />
                        Compatible
                      </div>
                    </div>
                    <div className='token-mascot-panel'>
                      <div className='token-mascot-card'>
                        <img src='/token-dazi-logo.svg' alt='' />
                      </div>
                      <div className='token-barcode' aria-hidden='true' />
                    </div>
                  </div>
                </div>
              </div>

              <div className='token-shell token-metrics'>
                {METRICS.map((metric) => (
                  <div
                    className={`token-metric token-metric-${metric.accent}`}
                    data-stamp={metric.stamp}
                    key={metric.stamp}
                  >
                    <em>{metric.label}</em>
                    <b>{metric.value}</b>
                    <span>{metric.copy}</span>
                  </div>
                ))}
              </div>
            </section>

            <section className='token-section token-section-compact'>
              <div className='token-shell token-api-demo'>
                <div className='token-demo-left'>
                  <div className='token-kicker'>
                    <span />1 分钟接入
                  </div>
                  <h2 className='token-demo-title'>
                    <span>会用 OpenAI，</span>
                    <span>
                      就会用 <em>Token 搭子</em>。
                    </span>
                  </h2>
                  <p className='token-demo-subline'>
                    修改一行地址，模型统一接入、应用无缝衔接。
                  </p>
                  <div className='token-lang-switch' aria-label='代码语言'>
                    {[
                      ['python', 'Python'],
                      ['curl', 'cURL'],
                      ['js', 'JS'],
                      ['go', 'Go'],
                      ['rust', 'Rust'],
                    ].map(([key, label]) => (
                      <button
                        aria-pressed={activeLanguage === key}
                        className={
                          activeLanguage === key ? 'is-active' : undefined
                        }
                        key={key}
                        onClick={() => setActiveLanguage(key)}
                        type='button'
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className='token-mobile-url' aria-label='手机端接口地址'>
                  <div>
                    <strong translate='no'>{apiBaseURL}</strong>
                    <button
                      type='button'
                      onClick={() => handleCopyBaseURL('mobile')}
                      aria-label='复制 Token 搭子 API URL'
                    >
                      {copiedButton === 'mobile' ? '已复制' : '复制'}
                    </button>
                  </div>
                </div>

                <div className='token-code-window' aria-label='接入代码示例'>
                  <div className='token-code-head'>
                    <div className='token-window-meta'>
                      <div className='token-mac-dots' aria-hidden='true'>
                        <span style={{ '--dot': '#ff5f57' }} />
                        <span style={{ '--dot': '#ffbd2e' }} />
                        <span style={{ '--dot': '#28c840' }} />
                      </div>
                      <strong>{activeSample.title}</strong>
                    </div>
                    <code>{apiBaseHost}</code>
                  </div>
                  <pre>
                    <code
                      dangerouslySetInnerHTML={{ __html: activeSample.code }}
                    />
                  </pre>
                </div>
              </div>
            </section>

            <section className='token-section token-dark-band'>
              <div className='token-shell'>
                <div className='token-section-head'>
                  <div>
                    <div className='token-section-kicker'>
                      Model Passport for Vibe Coding
                    </div>
                    <h2>灵感一闪，代码跟上。</h2>
                  </div>
                  <p>
                    Token
                    搭子帮你处理模型切换、线路连接和用量记录。现在，请进入你的创作心流（Vibe
                    Coding）。
                  </p>
                </div>
                <p className='token-steps-intro'>只需 3 步，30秒极速开工：</p>
                <div className='token-steps'>
                  {STEPS.map(([number, title, copyText]) => (
                    <div className='token-step' key={number}>
                      <div className='token-step-head'>
                        <code>{number}</code>
                        <h3>{title}</h3>
                      </div>
                      <p>{copyText}</p>
                    </div>
                  ))}
                </div>
              </div>
            </section>

            <section className='token-cta'>
              <div className='token-shell token-cta-box'>
                <div>
                  <h2>你的模型搭子，7x24 小时在线。</h2>
                  <p>
                    一把密钥，召唤多路顶尖模型。把繁琐的接管抛在脑后，让灵感与代码同步输出。
                  </p>
                </div>
                <Link className='token-btn token-btn-primary' to='/register'>
                  🚀 获取通行证，马上开工 <i aria-hidden='true'>›</i>
                </Link>
              </div>
            </section>
          </main>
          <footer className='token-footer'>
            <div className='token-shell'>
              <span>Token 搭子 · Model Passport for Vibe Coding</span>
              <span>tokendazi.com</span>
            </div>
          </footer>
        </div>
      ) : (
        <div className='classic-page-fill overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-full border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
