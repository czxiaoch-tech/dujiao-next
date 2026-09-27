<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { rechargePortalAPI } from '../api/rechargePortal'

type Step = 1 | 2 | 3 | 4
type ResultStatus = 'pending' | 'processing' | 'completed' | 'failed'

const RESULT_TOKEN_KEY = 'aishopone_recharge_result_token_v1'

const step = ref<Step>(1)
const loading = ref(false)
const error = ref('')
const code = ref('')
const sessionJSON = ref('')
const redemptionToken = ref('')
const preflightToken = ref('')
const resultToken = ref('')
const orderNo = ref('')
const productTitle = ref('ChatGPT Pro 20X')
const accountEmail = ref('')
const accountID = ref('')
const resultStatus = ref<ResultStatus>('pending')
const resultMessage = ref('')

let pollTimer: number | undefined
let pollStartedAt = 0

const progress = computed(() => [
  { n: 1, label: '验证卡密' },
  { n: 2, label: '填写充值资料' },
  { n: 3, label: '确认充值' },
  { n: 4, label: '查看结果' },
])

const statusTitle = computed(() => {
  switch (resultStatus.value) {
    case 'completed': return '充值已完成'
    case 'failed': return '充值未完成'
    case 'processing': return '正在充值'
    default: return '等待处理'
  }
})

const statusHint = computed(() => {
  switch (resultStatus.value) {
    case 'completed': return resultMessage.value || '订单已经完成。'
    case 'failed': return resultMessage.value || '本次充值未完成，请联系本站客服处理。'
    case 'processing': return '系统正在处理，请保持本页打开。'
    default: return '订单已提交，正在等待处理。'
  }
})

function apiPayload(response: any) {
  return response?.data?.data ?? response?.data ?? {}
}

function resetError() {
  error.value = ''
}

async function previewCode() {
  resetError()
  if (!code.value.trim()) {
    error.value = '请输入 AI Shop One 卡密'
    return
  }
  loading.value = true
  try {
    const response = await rechargePortalAPI.preview(code.value.trim())
    const data = apiPayload(response)
    redemptionToken.value = String(data.redemption_token || '')
    const title = data.product_title || {}
    productTitle.value = String(title['zh-CN'] || title['en-US'] || 'ChatGPT Pro 20X')
    if (!redemptionToken.value) throw new Error('卡密验证结果异常')
    step.value = 2
  } catch (err: any) {
    error.value = err?.message || '卡密验证失败'
  } finally {
    loading.value = false
  }
}

async function preflight() {
  resetError()
  const raw = sessionJSON.value.trim()
  if (!raw) {
    error.value = '请粘贴完整 ChatGPT Session JSON'
    return
  }
  loading.value = true
  try {
    const response = await rechargePortalAPI.preflight(redemptionToken.value, raw)
    const data = apiPayload(response)
    preflightToken.value = String(data.preflight_token || '')
    accountEmail.value = String(data.email || '')
    accountID.value = String(data.account_id || '')
    if (!preflightToken.value) throw new Error('预检结果异常')
    sessionJSON.value = ''
    step.value = 3
  } catch (err: any) {
    error.value = err?.message || '充值资料预检失败'
  } finally {
    loading.value = false
  }
}

async function confirmRedeem() {
  resetError()
  loading.value = true
  try {
    const response = await rechargePortalAPI.redeem(preflightToken.value)
    const data = apiPayload(response)
    resultToken.value = String(data.result_token || '')
    orderNo.value = String(data.order_no || '')
    resultStatus.value = (String(data.status || 'processing') as ResultStatus)
    if (!resultToken.value) throw new Error('订单提交结果异常')
    sessionStorage.setItem(RESULT_TOKEN_KEY, resultToken.value)
    step.value = 4
    startPolling()
  } catch (err: any) {
    error.value = err?.message || '提交充值失败'
  } finally {
    loading.value = false
  }
}

async function loadResult() {
  if (!resultToken.value) return
  try {
    const response = await rechargePortalAPI.result(resultToken.value)
    const data = apiPayload(response)
    orderNo.value = String(data.order_no || orderNo.value)
    resultStatus.value = (String(data.status || 'pending') as ResultStatus)
    resultMessage.value = String(data.message || '')
    if (resultStatus.value === 'completed' || resultStatus.value === 'failed') {
      stopPolling()
    }
  } catch {
    // 查询失败时保留当前状态，下次轮询继续。
  }
}

function startPolling() {
  stopPolling()
  pollStartedAt = Date.now()
  void loadResult()
  pollTimer = window.setInterval(() => {
    if (Date.now() - pollStartedAt > 30 * 60 * 1000) {
      stopPolling()
      return
    }
    void loadResult()
  }, 3000)
}

function stopPolling() {
  if (pollTimer) {
    window.clearInterval(pollTimer)
    pollTimer = undefined
  }
}

function startOver() {
  stopPolling()
  sessionStorage.removeItem(RESULT_TOKEN_KEY)
  step.value = 1
  code.value = ''
  sessionJSON.value = ''
  redemptionToken.value = ''
  preflightToken.value = ''
  resultToken.value = ''
  orderNo.value = ''
  accountEmail.value = ''
  accountID.value = ''
  resultStatus.value = 'pending'
  resultMessage.value = ''
  error.value = ''
}

onMounted(() => {
  const saved = sessionStorage.getItem(RESULT_TOKEN_KEY)
  if (saved) {
    resultToken.value = saved
    step.value = 4
    startPolling()
  }
})

onUnmounted(stopPolling)
</script>

<template>
  <main class="recharge-page">
    <section class="recharge-shell">
      <header class="hero">
        <div class="brand-mark">AI</div>
        <div>
          <p class="eyebrow">AI Shop One</p>
          <h1>卡密充值中心</h1>
          <p class="subtitle">本站卡密 · 本站提交 · 本站查询</p>
        </div>
      </header>

      <div class="steps" aria-label="充值步骤">
        <div v-for="item in progress" :key="item.n" class="step-item" :class="{ active: step === item.n, done: step > item.n }">
          <span class="step-dot">{{ item.n }}</span>
          <span>{{ item.label }}</span>
        </div>
      </div>

      <section class="panel">
        <div v-if="step === 1" class="stage">
          <div class="stage-head">
            <span class="stage-index">01</span>
            <div>
              <h2>验证 AI Shop One 卡密</h2>
              <p>输入购买后获得的本站卡密，系统会自动识别对应商品。</p>
            </div>
          </div>
          <label class="field">
            <span>卡密</span>
            <input v-model="code" class="input" autocomplete="off" placeholder="请输入 AI Shop One 卡密" @keyup.enter="previewCode" />
          </label>
          <button class="primary-btn" :disabled="loading" @click="previewCode">
            {{ loading ? '验证中…' : '验证卡密' }}
          </button>
        </div>

        <div v-else-if="step === 2" class="stage">
          <div class="stage-head">
            <span class="stage-index">02</span>
            <div>
              <h2>填写充值资料</h2>
              <p>已识别：<strong>{{ productTitle }}</strong></p>
            </div>
          </div>

          <div class="notice">
            打开
            <a href="https://chatgpt.com/api/auth/session" target="_blank" rel="noopener">ChatGPT Session 页面</a>
            ，复制页面显示的完整 JSON 后粘贴到下方。不要只粘贴 Access Token。
          </div>

          <label class="field">
            <span>ChatGPT Session JSON</span>
            <textarea
              v-model="sessionJSON"
              class="input textarea"
              spellcheck="false"
              autocomplete="off"
              placeholder='{"user":{...},"accessToken":"...","sessionToken":"..."}'
            />
          </label>
          <p class="security-note">提交后会先加密保存；页面进入下一步后，本浏览器中的 Session 文本会立即清空。</p>
          <div class="actions">
            <button class="secondary-btn" :disabled="loading" @click="step = 1">返回</button>
            <button class="primary-btn" :disabled="loading" @click="preflight">
              {{ loading ? '预检中…' : '预检充值资料' }}
            </button>
          </div>
        </div>

        <div v-else-if="step === 3" class="stage">
          <div class="stage-head">
            <span class="stage-index">03</span>
            <div>
              <h2>确认充值</h2>
              <p>请确认商品与账号信息，再提交正式充值。</p>
            </div>
          </div>

          <div class="summary-card">
            <div><span>商品</span><strong>{{ productTitle }}</strong></div>
            <div v-if="accountEmail"><span>账号</span><strong>{{ accountEmail }}</strong></div>
            <div v-if="accountID"><span>Account ID</span><code>{{ accountID }}</code></div>
            <div><span>执行方式</span><strong>自动充值</strong></div>
          </div>

          <div class="warning">确认后本站卡密会被正式使用，并生成充值订单。请勿重复提交。</div>

          <div class="actions">
            <button class="secondary-btn" :disabled="loading" @click="step = 2">返回</button>
            <button class="primary-btn" :disabled="loading" @click="confirmRedeem">
              {{ loading ? '提交中…' : '确认并开始充值' }}
            </button>
          </div>
        </div>

        <div v-else class="stage result-stage">
          <div class="result-icon" :class="resultStatus">
            <span v-if="resultStatus === 'completed'">✓</span>
            <span v-else-if="resultStatus === 'failed'">!</span>
            <span v-else class="spinner" />
          </div>
          <h2>{{ statusTitle }}</h2>
          <p class="result-hint">{{ statusHint }}</p>

          <div v-if="orderNo" class="order-box">
            <span>订单号</span>
            <code>{{ orderNo }}</code>
          </div>

          <div class="status-pill" :class="resultStatus">
            {{ resultStatus === 'completed' ? '已完成' : resultStatus === 'failed' ? '需要处理' : '处理中' }}
          </div>

          <button v-if="resultStatus === 'completed' || resultStatus === 'failed'" class="secondary-btn standalone" @click="startOver">
            兑换另一张卡密
          </button>
        </div>

        <p v-if="error" class="error-box">{{ error }}</p>
      </section>

      <footer class="footer-note">
        AI Shop One 卡密充值中心 · 充值过程中请勿重复提交
      </footer>
    </section>
  </main>
</template>

<style scoped>
.recharge-page {
  min-height: calc(100vh - 80px);
  padding: 48px 20px 72px;
  background:
    radial-gradient(circle at 15% 10%, color-mix(in srgb, var(--primary, #2563eb) 10%, transparent), transparent 34%),
    var(--bg, #f6f7f9);
}
.recharge-shell { max-width: 820px; margin: 0 auto; }
.hero { display: flex; align-items: center; gap: 18px; margin-bottom: 30px; }
.brand-mark {
  width: 58px; height: 58px; border-radius: 18px; display: grid; place-items: center;
  background: var(--primary, #2563eb); color: white; font-weight: 800; font-size: 20px;
  box-shadow: 0 12px 30px color-mix(in srgb, var(--primary, #2563eb) 28%, transparent);
}
.eyebrow { margin: 0 0 3px; color: var(--primary, #2563eb); font-weight: 700; font-size: 13px; letter-spacing: .08em; text-transform: uppercase; }
h1 { margin: 0; font-size: clamp(28px, 5vw, 40px); color: var(--ink, #111827); line-height: 1.1; }
.subtitle { margin: 7px 0 0; color: var(--ink-3, #6b7280); }
.steps {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px;
  margin-bottom: 18px;
}
.step-item {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  min-height: 46px; padding: 8px 10px; border: 1px solid var(--brd, #e5e7eb);
  border-radius: 13px; background: var(--surface, #fff); color: var(--ink-3, #6b7280);
  font-size: 13px; font-weight: 600;
}
.step-item.active { border-color: var(--primary, #2563eb); color: var(--primary, #2563eb); }
.step-item.done { color: var(--good, #16a34a); }
.step-dot {
  width: 24px; height: 24px; border-radius: 999px; display: grid; place-items: center;
  background: var(--soft, #f3f4f6); font-size: 12px;
}
.step-item.active .step-dot { background: var(--primary, #2563eb); color: white; }
.panel {
  background: var(--surface, #fff); border: 1px solid var(--brd, #e5e7eb);
  border-radius: 24px; padding: clamp(24px, 5vw, 42px);
  box-shadow: 0 22px 70px rgba(15, 23, 42, .08);
}
.stage { display: grid; gap: 22px; }
.stage-head { display: flex; gap: 16px; align-items: flex-start; }
.stage-index {
  flex: 0 0 auto; font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: var(--primary, #2563eb); font-size: 13px; font-weight: 700;
  border: 1px solid color-mix(in srgb, var(--primary, #2563eb) 30%, var(--brd, #e5e7eb));
  border-radius: 999px; padding: 5px 8px;
}
.stage h2 { margin: 0; color: var(--ink, #111827); font-size: 24px; }
.stage-head p { margin: 7px 0 0; color: var(--ink-3, #6b7280); line-height: 1.65; }
.field { display: grid; gap: 8px; color: var(--ink-2, #374151); font-size: 14px; font-weight: 600; }
.input {
  width: 100%; box-sizing: border-box; border: 1px solid var(--brd, #d1d5db);
  background: var(--surface, #fff); color: var(--ink, #111827); border-radius: 14px;
  padding: 14px 15px; outline: none; font: inherit; transition: .18s ease;
}
.input:focus { border-color: var(--primary, #2563eb); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary, #2563eb) 12%, transparent); }
.textarea { min-height: 190px; resize: vertical; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; line-height: 1.55; }
.primary-btn, .secondary-btn {
  min-height: 48px; border-radius: 14px; padding: 0 20px; font-weight: 700; cursor: pointer;
  transition: transform .12s ease, opacity .12s ease; border: 0;
}
.primary-btn { background: var(--primary, #2563eb); color: white; }
.secondary-btn { background: var(--soft, #f3f4f6); color: var(--ink, #111827); border: 1px solid var(--brd, #e5e7eb); }
.primary-btn:hover:not(:disabled), .secondary-btn:hover:not(:disabled) { transform: translateY(-1px); }
.primary-btn:disabled, .secondary-btn:disabled { opacity: .55; cursor: wait; }
.actions { display: grid; grid-template-columns: 1fr 2fr; gap: 12px; }
.notice, .warning, .security-note, .error-box {
  border-radius: 14px; line-height: 1.65; font-size: 13px;
}
.notice { padding: 14px 16px; background: color-mix(in srgb, var(--primary, #2563eb) 7%, var(--surface, #fff)); color: var(--ink-2, #374151); }
.notice a { color: var(--primary, #2563eb); font-weight: 700; }
.warning { padding: 14px 16px; background: #fff7ed; color: #9a3412; border: 1px solid #fed7aa; }
.security-note { margin: -10px 0 0; color: var(--ink-3, #6b7280); }
.error-box { margin: 20px 0 0; padding: 12px 14px; background: #fef2f2; color: #b91c1c; border: 1px solid #fecaca; }
.summary-card {
  display: grid; border: 1px solid var(--brd, #e5e7eb); border-radius: 16px; overflow: hidden;
}
.summary-card > div { display: grid; grid-template-columns: 130px 1fr; gap: 18px; padding: 14px 16px; border-bottom: 1px solid var(--brd, #e5e7eb); }
.summary-card > div:last-child { border-bottom: 0; }
.summary-card span { color: var(--ink-3, #6b7280); }
.summary-card strong, .summary-card code { color: var(--ink, #111827); overflow-wrap: anywhere; }
.result-stage { text-align: center; justify-items: center; padding: 20px 0 6px; }
.result-icon {
  width: 72px; height: 72px; border-radius: 999px; display: grid; place-items: center;
  font-size: 34px; font-weight: 800; background: #eff6ff; color: #2563eb;
}
.result-icon.completed { background: #ecfdf5; color: #059669; }
.result-icon.failed { background: #fef2f2; color: #dc2626; }
.spinner { width: 28px; height: 28px; border-radius: 999px; border: 3px solid currentColor; border-right-color: transparent; animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.result-hint { margin: -10px 0 0; color: var(--ink-3, #6b7280); line-height: 1.7; }
.order-box { display: flex; gap: 14px; align-items: center; padding: 12px 16px; border-radius: 12px; background: var(--soft, #f3f4f6); }
.order-box span { color: var(--ink-3, #6b7280); }
.order-box code { color: var(--ink, #111827); }
.status-pill { border-radius: 999px; padding: 7px 14px; background: #eff6ff; color: #1d4ed8; font-size: 13px; font-weight: 700; }
.status-pill.completed { background: #ecfdf5; color: #047857; }
.status-pill.failed { background: #fef2f2; color: #b91c1c; }
.standalone { min-width: 200px; }
.footer-note { margin: 18px 0 0; text-align: center; color: var(--ink-3, #6b7280); font-size: 12px; }

@media (max-width: 680px) {
  .recharge-page { padding: 28px 14px 48px; }
  .steps { grid-template-columns: repeat(2, 1fr); }
  .step-item { justify-content: flex-start; }
  .panel { border-radius: 18px; padding: 22px 18px; }
  .actions { grid-template-columns: 1fr; }
  .summary-card > div { grid-template-columns: 1fr; gap: 4px; }
}
</style>
